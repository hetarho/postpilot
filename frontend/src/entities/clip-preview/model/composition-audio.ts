import type { ClipEditPlan } from '@/entities/clip-plan/@x/clip-preview'
import type { ClipSpeechRef } from '@/entities/clip-plan/@x/clip-preview'
import { CLIP_BROWSER_RENDER } from '@/entities/clip-design/@x/clip-preview'
import {
  canonicalSelectedAudio,
  createAudioProcessor,
  createAudioRangeReader,
  type BrowserMediaSourceAccess,
  type OriginalAudioMetadata,
  type PcmChannels,
} from '@/shared/lib'
import { CLIP_AUDIO_PROCESSING } from '../config/audio-processing'
import { browserAudioPlan } from './browser-audio-plan'
import { canonicalSpeechBuffer, type SpeechAudioLoader } from './speech-playback'

export class BrowserAudioCompositionError extends Error {
  constructor(
    readonly reason: 'speech' | 'memory' | 'capability' | 'audio',
    readonly segmentId = '',
  ) {
    super(`CLIP_BROWSER_AUDIO_${reason.toUpperCase()}`)
  }
}
export interface BrowserPreparedAudioSources {
  schedule: ReturnType<typeof browserAudioPlan>
  cuts: { cut: ReturnType<typeof browserAudioPlan>['cuts'][number]; channels: PcmChannels }[]
  sourceResources: {
    sourceReads: number
    sourceBytes: number
    sourceReadMs: number
    openMs: number
    decodeMs: number
    peakRangePcmBytes: number
    originals: OriginalAudioMetadata[]
    physicalPeakBytes: null
  }
}
/** Shared selected-range/resampler/time-stretch semantics; final loudness belongs to export. */
export async function prepareBrowserAudioSources(
  plan: ClipEditPlan,
  source: (fingerprint: string, signal: AbortSignal) => Promise<BrowserMediaSourceAccess>,
  signal: AbortSignal,
  options: { strict?: boolean } = {},
): Promise<BrowserPreparedAudioSources> {
  signal.throwIfAborted()
  const schedule = browserAudioPlan(plan)
  if (
    options.strict !== false &&
    plan.narration?.enabled &&
    (!schedule.speech.length || schedule.speechIssues.length)
  )
    throw new BrowserAudioCompositionError('speech', schedule.speechIssues[0]?.segmentId)
  const sourceResources: BrowserPreparedAudioSources['sourceResources'] = {
    sourceReads: 0,
    sourceBytes: 0,
    sourceReadMs: 0,
    openMs: 0,
    decodeMs: 0,
    peakRangePcmBytes: 0,
    originals: [],
    physicalPeakBytes: null,
  }
  const cuts: BrowserPreparedAudioSources['cuts'] = []
  const reserved =
    schedule.sampleFrames * 2 * 4 * CLIP_AUDIO_PROCESSING.mixCopies +
    schedule.cuts.reduce((total, cut) => total + cut.frames * 2 * 4, 0) +
    schedule.speech.reduce(
      (total, speech) =>
        total +
        Math.ceil(speech.duration * schedule.sampleRate) *
          2 *
          4 *
          CLIP_AUDIO_PROCESSING.speechCopies,
      0,
    ) +
    CLIP_AUDIO_PROCESSING.dspWorkingBytes +
    CLIP_AUDIO_PROCESSING.encodedPacketBytes
  const sourceFrames = Math.max(0, ...schedule.cuts.map((cut) => cut.sourceFrames))
  const guard = Math.ceil(
    ((CLIP_AUDIO_PROCESSING.decoderPrerollMs + CLIP_AUDIO_PROCESSING.decoderTailMs) *
      schedule.sampleRate) /
      1000,
  )
  const rangePcmBytes = Math.floor(
    (CLIP_BROWSER_RENDER.audioDecodedBytes -
      reserved -
      (sourceFrames + guard) * 2 * 4 * CLIP_AUDIO_PROCESSING.canonicalRangeCopies) /
      CLIP_AUDIO_PROCESSING.transientRangeCopies,
  )
  if (
    !Number.isSafeInteger(reserved) ||
    reserved > CLIP_BROWSER_RENDER.audioDecodedBytes ||
    (schedule.cuts.length && rangePcmBytes <= 0)
  )
    throw new BrowserAudioCompositionError('memory')
  if (!schedule.cuts.length) return { schedule, cuts, sourceResources }
  const reader = createAudioRangeReader(signal, CLIP_AUDIO_PROCESSING)
  const processor = createAudioProcessor(signal, {
    maxPcmBytes: CLIP_AUDIO_PROCESSING.maxPcmBytes,
    maxEncodedBytes: CLIP_AUDIO_PROCESSING.encodedPacketBytes,
    maxEncodedPackets: CLIP_AUDIO_PROCESSING.encodedPackets,
    maxPacketBytes: CLIP_AUDIO_PROCESSING.encodedPacketMaxBytes,
    maxPrimingFrames: CLIP_AUDIO_PROCESSING.encoderPaddingFrames,
    operationTimeoutMs: CLIP_AUDIO_PROCESSING.operationTimeoutMs,
    cleanupTimeoutMs: CLIP_AUDIO_PROCESSING.cleanupTimeoutMs,
  })
  try {
    for (const cut of schedule.cuts) {
      signal.throwIfAborted()
      const access = await source(cut.fingerprint, signal)
      signal.throwIfAborted()
      const range = await reader.decode(
        access,
        {
          startUs: cut.sourceStartUs,
          endUs: cut.sourceEndUs,
          targetSampleRate: schedule.sampleRate,
        },
        rangePcmBytes,
      )
      if (!range) continue
      sourceResources.sourceReads += range.measurements.sourceReads
      sourceResources.sourceBytes += range.measurements.sourceBytes
      sourceResources.sourceReadMs += range.measurements.sourceReadMs
      sourceResources.openMs += range.measurements.openMs
      sourceResources.decodeMs += range.measurements.decodeMs
      sourceResources.peakRangePcmBytes = Math.max(
        sourceResources.peakRangePcmBytes,
        range.measurements.logicalPcmBytes + range.measurements.decoderReservedBytes,
      )
      sourceResources.originals.push(range.metadata)
      const selected = await canonicalSelectedAudio(
        range,
        cut.sourceStart,
        cut.sourceFrames,
        schedule.sampleRate,
        signal,
      )
      const channels = await processor.stretch(
        selected,
        schedule.sampleRate,
        cut.rate,
        cut.frames,
        cut.volume,
      )
      signal.throwIfAborted()
      cuts.push({ cut, channels })
    }
    return { schedule, cuts, sourceResources }
  } catch (error) {
    cuts.length = 0
    throw error
  } finally {
    reader.close()
    processor.close()
  }
}
/** Immutable speech hash and codec priming are verified identically for preview and export. */
export async function loadVerifiedSpeechBuffer(
  context: BaseAudioContext,
  speech: ClipSpeechRef,
  load: SpeechAudioLoader,
  signal: AbortSignal,
): Promise<AudioBuffer> {
  const bytes = await load(speech, signal)
  signal.throwIfAborted()
  if (!bytes.byteLength || bytes.byteLength > CLIP_BROWSER_RENDER.speechEncodedBytes)
    throw new BrowserAudioCompositionError('audio')
  const digest = await crypto.subtle.digest('SHA-256', bytes)
  signal.throwIfAborted()
  if (
    [...new Uint8Array(digest)].map((value) => value.toString(16).padStart(2, '0')).join('') !==
    speech.audioHash
  )
    throw new BrowserAudioCompositionError('audio')
  const decoded = await context.decodeAudioData(bytes)
  signal.throwIfAborted()
  return canonicalSpeechBuffer(context, speech, decoded)
}
