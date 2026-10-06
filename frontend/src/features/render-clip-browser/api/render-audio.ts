import { type ClipEditPlan } from '@/entities/clip-plan'
import {
  canonicalSpeechBuffer,
  clipBrowserEncoderConfig,
  speechDecodeKey,
  SpeechDecodeCache,
  CLIP_AUDIO_PROCESSING,
  speechRenderFingerprint,
  type SpeechAudioLoader,
} from '@/entities/clip-preview'
import { type ClipRatio } from '@/entities/clip-project'
import { CLIP_BROWSER_RENDER, CLIP_DESIGN } from '@/entities/clip-design'
import {
  createAudioProcessor,
  createAudioRangeReader,
  type EncodedAudioTrack,
  type PcmChannels,
  type OriginalAudioMetadata,
} from '@/shared/lib'
import type { BrowserOriginals } from '../lib/originals'
import { BrowserAudioRenderError, browserAudioPreflight } from '../model/audio-preflight'
import { canonicalSelectedAudio } from '../lib/selected-audio'

export interface BrowserAudioTrack extends EncodedAudioTrack {
  speechFingerprint?: string
  loudnessLUFS?: number
  truePeakDBTP: number
  silent: boolean
  sourceResources?: {
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
export async function renderBrowserAudio(
  plan: ClipEditPlan,
  ratio: ClipRatio,
  originals: Pick<BrowserOriginals, 'source'>,
  signal: AbortSignal,
  progress?: (completed: number, total: number) => void,
  loadSpeech?: SpeechAudioLoader,
): Promise<BrowserAudioTrack | undefined> {
  signal.throwIfAborted()
  const { schedule, rangePcmBytes } = browserAudioPreflight(plan)
  if (!schedule.cuts.length && !schedule.speech.length) return undefined
  if (schedule.speech.length && !loadSpeech) throw new BrowserAudioRenderError('speech')
  if (
    typeof AudioContext === 'undefined' ||
    typeof OfflineAudioContext === 'undefined' ||
    typeof AudioEncoder === 'undefined'
  )
    throw new BrowserAudioRenderError('capability')
  const support = await AudioEncoder.isConfigSupported(clipBrowserEncoderConfig(ratio).audio)
  signal.throwIfAborted()
  if (!support.supported) throw new BrowserAudioRenderError('capability')
  let decoder: AudioContext
  try {
    decoder = new AudioContext({ sampleRate: schedule.sampleRate })
    if (decoder.sampleRate !== schedule.sampleRate) {
      await decoder.close()
      throw new BrowserAudioRenderError('capability')
    }
  } catch {
    throw new BrowserAudioRenderError('capability')
  }
  const nodes: AudioNode[] = []
  let processor: ReturnType<typeof createAudioProcessor> | undefined
  let rangeReader: ReturnType<typeof createAudioRangeReader> | undefined
  let hasAudio = false
  const sourceResources: NonNullable<BrowserAudioTrack['sourceResources']> = {
    sourceReads: 0,
    sourceBytes: 0,
    sourceReadMs: 0,
    openMs: 0,
    decodeMs: 0,
    peakRangePcmBytes: 0,
    originals: [],
    physicalPeakBytes: null,
  }
  const speechCache = new SpeechDecodeCache()
  try {
    processor = createAudioProcessor(signal, {
      maxPcmBytes: CLIP_AUDIO_PROCESSING.maxPcmBytes,
      maxEncodedBytes: CLIP_AUDIO_PROCESSING.encodedPacketBytes,
      maxEncodedPackets: CLIP_AUDIO_PROCESSING.encodedPackets,
      maxPacketBytes: CLIP_AUDIO_PROCESSING.encodedPacketMaxBytes,
      maxPrimingFrames: CLIP_AUDIO_PROCESSING.encoderPaddingFrames,
      operationTimeoutMs: CLIP_AUDIO_PROCESSING.operationTimeoutMs,
      cleanupTimeoutMs: CLIP_AUDIO_PROCESSING.cleanupTimeoutMs,
    })
    if (schedule.cuts.length) rangeReader = createAudioRangeReader(signal, CLIP_AUDIO_PROCESSING)
    const context = new OfflineAudioContext(2, schedule.sampleFrames, schedule.sampleRate)
    const master = context.createGain()
    master.gain.setValueAtTime(1, 0)
    for (const dip of schedule.dip) {
      master.gain.setValueAtTime(10 ** (CLIP_DESIGN.audio.hook_dip_db / 20), dip.start)
      master.gain.setValueAtTime(1, dip.end)
    }
    master.connect(context.destination)
    nodes.push(master)
    for (const cut of schedule.cuts) {
      signal.throwIfAborted()
      const access = await originals.source(cut.fingerprint)
      signal.throwIfAborted()
      const range = await rangeReader!.decode(
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
      hasAudio = true
      const channels = await canonicalSelectedAudio(
        range,
        cut.sourceStart,
        cut.sourceFrames,
        schedule.sampleRate,
        signal,
      )
      const stretched = await processor.stretch(
        channels,
        schedule.sampleRate,
        cut.rate,
        cut.frames,
        cut.volume,
      )
      signal.throwIfAborted()
      const audio = context.createBuffer(2, cut.frames, schedule.sampleRate)
      stretched.forEach((channel, index) => audio.copyToChannel(channel, index))
      const source = context.createBufferSource(),
        gain = context.createGain()
      source.buffer = audio
      gain.gain.setValueAtTime(cut.fadeIn ? 0 : 1, cut.start)
      if (cut.fadeIn) gain.gain.linearRampToValueAtTime(1, cut.start + cut.fadeIn)
      if (cut.fadeOut) {
        gain.gain.setValueAtTime(1, cut.fadeOutStart)
        gain.gain.linearRampToValueAtTime(0, cut.fadeOutStart + cut.fadeOut)
      }
      source.connect(gain).connect(master)
      source.start(cut.start)
      nodes.push(source, gain)
    }
    for (const spoken of schedule.speech) {
      signal.throwIfAborted()
      const key = speechDecodeKey(spoken.speech)
      let buffer = speechCache.get(key)
      if (!buffer) {
        try {
          const encoded = await loadSpeech!(spoken.speech, signal)
          signal.throwIfAborted()
          if (!encoded.byteLength || encoded.byteLength > CLIP_BROWSER_RENDER.speechEncodedBytes)
            throw new BrowserAudioRenderError('audio', spoken.segmentId)
          const digest = await crypto.subtle.digest('SHA-256', encoded)
          const hash = Array.from(new Uint8Array(digest), (v) =>
            v.toString(16).padStart(2, '0'),
          ).join('')
          if (hash !== spoken.speech.audioHash)
            throw new BrowserAudioRenderError('audio', spoken.segmentId)
          buffer = canonicalSpeechBuffer(
            context,
            spoken.speech,
            await decoder.decodeAudioData(encoded),
          )
          signal.throwIfAborted()
          speechCache.put(key, buffer)
        } catch (error) {
          signal.throwIfAborted()
          if (error instanceof BrowserAudioRenderError) throw error
          throw new BrowserAudioRenderError('audio', spoken.segmentId)
        }
      }
      const source = context.createBufferSource(),
        gain = context.createGain()
      source.buffer = buffer
      gain.gain.value = schedule.narrationVolume
      source.connect(gain).connect(context.destination)
      source.start(spoken.start)
      nodes.push(source, gain)
      hasAudio = true
    }
    if (!hasAudio) return undefined
    signal.throwIfAborted()
    const mixed = await context.startRendering()
    signal.throwIfAborted()
    const channels: PcmChannels = [0, 1].map((channel) => mixed.getChannelData(channel).slice())
    const normalized = await processor.normalize(
      channels,
      CLIP_DESIGN.audio.loudnorm.i,
      CLIP_DESIGN.audio.loudnorm.tp,
    )
    const track = await processor.encode(
      normalized.channels,
      clipBrowserEncoderConfig(ratio).audio,
      CLIP_BROWSER_RENDER.audioBatchFrames,
      CLIP_BROWSER_RENDER.encodeQueueFrames,
      progress,
    )
    if (
      !track.silent &&
      (!Number.isFinite(track.loudnessLUFS) ||
        Math.abs(track.loudnessLUFS! - CLIP_DESIGN.audio.loudnorm.i) > 1 ||
        !Number.isFinite(track.truePeakDBTP) ||
        track.truePeakDBTP > CLIP_DESIGN.audio.loudnorm.tp)
    ) {
      track.chunks.length = 0
      throw new BrowserAudioRenderError('audio')
    }
    return { ...track, sourceResources, speechFingerprint: await speechRenderFingerprint(plan) }
  } finally {
    processor?.close()
    rangeReader?.close()
    for (const node of nodes) node.disconnect()
    speechCache.clear()
    await decoder.close()
  }
}
