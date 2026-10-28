import { CLIP_BROWSER_RENDER, CLIP_DESIGN } from '@/entities/clip-design/@x/clip-preview'
import type { EncoderSupport } from '@/shared/lib'
import type { ClipRatio } from '@/entities/clip-project/@x/clip-preview'
import type { ClipEditPlan } from '@/entities/clip-plan/@x/clip-preview'
import { clipSourceSound } from '@/entities/clip-plan/@x/clip-preview'

/** `unqualified`: the device could encode, but browser final rendering is not yet released
 *  (ARCH-68), so the server refuses its start. */
export type ClipBrowserRenderCapability =
  { available: true } | { available: false; reason: 'capability' | 'memory' | 'unqualified' }

export function clipBrowserRenderCapability(
  support: EncoderSupport,
  needsAudio: boolean,
): ClipBrowserRenderCapability {
  if (!support.video || (needsAudio && !support.audio)) {
    return { available: false, reason: 'capability' }
  }
  if (
    support.deviceMemoryGB !== undefined &&
    support.deviceMemoryGB < CLIP_BROWSER_RENDER.memoryFloorGB
  ) {
    return { available: false, reason: 'memory' }
  }
  return { available: true }
}

export function clipRenderNeedsAudio(plan: ClipEditPlan) {
  return !!plan.narration?.enabled || plan.cuts.some((cut) => clipSourceSound(plan, cut))
}

export function clipBrowserEncoderConfig(ratio: ClipRatio) {
  const video: VideoEncoderConfig = {
    codec: CLIP_BROWSER_RENDER.videoCodec,
    ...CLIP_DESIGN.ratios[ratio].canvas,
    framerate: CLIP_BROWSER_RENDER.frameRate,
    bitrate: CLIP_BROWSER_RENDER.videoBitrate,
    avc: { format: 'avc' },
  }
  const audio: AudioEncoderConfig = {
    codec: CLIP_BROWSER_RENDER.audioCodec,
    sampleRate: CLIP_BROWSER_RENDER.audioSampleRate,
    numberOfChannels: CLIP_BROWSER_RENDER.audioChannels,
    bitrate: CLIP_BROWSER_RENDER.audioBitrate,
  }
  return { video, audio }
}
