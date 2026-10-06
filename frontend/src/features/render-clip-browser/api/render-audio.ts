import { type ClipEditPlan } from '@/entities/clip-plan'
import {
  prepareBrowserAudioSources,
  loadVerifiedSpeechBuffer,
  BrowserAudioCompositionError,
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
  type EncodedAudioTrack,
  type PcmChannels,
  type OriginalAudioMetadata,
} from '@/shared/lib'
import type { BrowserOriginals } from '../lib/originals'
import { BrowserAudioRenderError, browserAudioPreflight } from '../model/audio-preflight'

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
  const { schedule } = browserAudioPreflight(plan)
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
    const context = new OfflineAudioContext(2, schedule.sampleFrames, schedule.sampleRate)
    const master = context.createGain()
    master.gain.setValueAtTime(1, 0)
    for (const dip of schedule.dip) {
      master.gain.setValueAtTime(10 ** (CLIP_DESIGN.audio.hook_dip_db / 20), dip.start)
      master.gain.setValueAtTime(1, dip.end)
    }
    master.connect(context.destination)
    nodes.push(master)
    const prepared = await prepareBrowserAudioSources(
      plan,
      (fingerprint) => originals.source(fingerprint),
      signal,
    )
    Object.assign(sourceResources, prepared.sourceResources)
    for (const { cut, channels: stretched } of prepared.cuts) {
      signal.throwIfAborted()
      hasAudio = true
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
          buffer = await loadVerifiedSpeechBuffer(decoder, spoken.speech, loadSpeech!, signal)
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
      CLIP_DESIGN.audio.loudnorm.lra,
    )
    const track = await processor.encode(
      normalized.channels,
      clipBrowserEncoderConfig(ratio).audio,
      CLIP_BROWSER_RENDER.audioBatchFrames,
      CLIP_BROWSER_RENDER.encodeQueueFrames,
      progress,
    )
    if (
      !Number.isFinite(track.loudnessRangeLU) ||
      track.loudnessRangeLU! > CLIP_DESIGN.audio.loudnorm.lra
    ) {
      track.chunks.length = 0
      throw new Error('AUDIO_LOUDNESS_RANGE_UNSUPPORTED')
    }
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
  } catch (error) {
    if (error instanceof BrowserAudioCompositionError)
      throw new BrowserAudioRenderError(error.reason, error.segmentId)
    throw error
  } finally {
    processor?.close()
    for (const node of nodes) node.disconnect()
    speechCache.clear()
    await decoder.close()
  }
}
