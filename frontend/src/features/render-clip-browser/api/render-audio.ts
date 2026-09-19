import { type ClipEditPlan } from '@/entities/clip-plan'
import { browserAudioPlan, clipBrowserEncoderConfig } from '@/entities/clip-preview'
import { type ClipRatio } from '@/entities/clip-project'
import { CLIP_BROWSER_RENDER, CLIP_DESIGN } from '@/entities/clip-design'
import {
  createAudioProcessor,
  mp4HasAudio,
  type EncodedAudioTrack,
  type PcmChannels,
} from '@/shared/lib'
import type { BrowserOriginals } from '../lib/originals'

export interface BrowserAudioTrack extends EncodedAudioTrack {
  loudnessLUFS?: number
  truePeakDBTP: number
  silent: boolean
}
export async function renderBrowserAudio(
  plan: ClipEditPlan,
  ratio: ClipRatio,
  originals: Pick<BrowserOriginals, 'get'>,
  signal: AbortSignal,
  progress?: (completed: number, total: number) => void,
): Promise<BrowserAudioTrack | undefined> {
  signal.throwIfAborted()
  const schedule = browserAudioPlan(plan)
  if (!schedule.cuts.length) return undefined
  const decoder = new AudioContext({ sampleRate: schedule.sampleRate })
  const decoded = new Map<string, AudioBuffer | undefined>()
  const nodes: AudioNode[] = []
  let processor: ReturnType<typeof createAudioProcessor> | undefined
  let hasAudio = false
  try {
    processor = createAudioProcessor(signal)
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
      if (!decoded.has(cut.fingerprint)) {
        const blob = await originals.get(cut.fingerprint)
        signal.throwIfAborted()
        let buffer: AudioBuffer | undefined
        if (await mp4HasAudio(blob, signal)) {
          buffer = await decoder.decodeAudioData(await blob.arrayBuffer())
          signal.throwIfAborted()
          if (buffer.numberOfChannels !== 2) {
            const stereo = new OfflineAudioContext(2, buffer.length, schedule.sampleRate)
            const source = stereo.createBufferSource(),
              gain = stereo.createGain()
            source.buffer = buffer
            // FFmpeg's mono-to-stereo rematrix preserves total channel energy.
            gain.gain.value = buffer.numberOfChannels === 1 ? Math.SQRT1_2 : 1
            source.connect(gain).connect(stereo.destination)
            source.start()
            buffer = await stereo.startRendering()
            source.disconnect()
            gain.disconnect()
          }
        }
        decoded.set(cut.fingerprint, buffer)
      }
      const buffer = decoded.get(cut.fingerprint)
      if (!buffer) continue
      hasAudio = true
      const channels: PcmChannels = [0, 1].map((channel) => {
        const samples = new Float32Array(cut.sourceFrames)
        samples.set(
          buffer
            .getChannelData(channel)
            .subarray(cut.sourceStart, cut.sourceStart + cut.sourceFrames),
        )
        return samples
      })
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
        gain.gain.setValueAtTime(1, cut.end - cut.fadeOut)
        gain.gain.linearRampToValueAtTime(0, cut.end)
      }
      source.connect(gain).connect(master)
      source.start(cut.start)
      nodes.push(source, gain)
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
    return track
  } finally {
    processor?.close()
    for (const node of nodes) node.disconnect()
    decoded.clear()
    await decoder.close()
  }
}
