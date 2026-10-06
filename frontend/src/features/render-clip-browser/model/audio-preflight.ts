import type { ClipEditPlan } from '@/entities/clip-plan'
import { browserAudioPlan, speechDecodeKey, CLIP_AUDIO_PROCESSING } from '@/entities/clip-preview'
import { CLIP_BROWSER_RENDER } from '@/entities/clip-design'
export class BrowserAudioRenderError extends Error {
  constructor(
    readonly reason: 'speech' | 'memory' | 'capability' | 'audio',
    readonly segmentId = '',
  ) {
    super(`CLIP_BROWSER_AUDIO_${reason.toUpperCase()}`)
  }
}
/** Reserve mix/normalization copies and both native/canonical immutable buffers before decoding. */
export function browserAudioPreflight(plan: ClipEditPlan) {
  const schedule = browserAudioPlan(plan)
  if (plan.narration?.enabled) {
    if (
      !plan.narration.confirmedVoiceId ||
      !plan.narration.bindingDigest ||
      !schedule.speech.length ||
      schedule.speechIssues.length
    )
      throw new BrowserAudioRenderError('speech', schedule.speechIssues[0]?.segmentId)
    let end = 0
    for (const segment of plan.narration.segments) {
      const speech = segment.speech!
      const frames = Math.ceil((speech.samples * schedule.sampleRate) / speech.sampleRate)
      if (
        !Number.isSafeInteger(frames) ||
        frames <= 0 ||
        speech.channels !== 2 ||
        speech.sampleRate !== 44100 ||
        segment.startMs < end ||
        (segment.startMs * schedule.sampleRate) / 1000 + frames > schedule.sampleFrames
      )
        throw new BrowserAudioRenderError('speech', segment.id)
      end = segment.endMs
    }
  }
  const unique = new Map(schedule.speech.map((s) => [speechDecodeKey(s.speech), s]))
  const largestSourceFrames = Math.max(0, ...schedule.cuts.map((c) => c.sourceFrames))
  const largestCutFrames = Math.max(0, ...schedule.cuts.map((c) => c.frames))
  const guardFrames = Math.ceil(
    ((CLIP_AUDIO_PROCESSING.decoderPrerollMs + CLIP_AUDIO_PROCESSING.decoderTailMs) *
      schedule.sampleRate) /
      1000,
  )
  const reservedBytes =
    schedule.sampleFrames * 2 * 4 * CLIP_AUDIO_PROCESSING.mixCopies +
    [...unique.values()].reduce(
      (total, s) =>
        total +
        Math.ceil(s.duration * schedule.sampleRate) * 2 * 4 * CLIP_AUDIO_PROCESSING.speechCopies,
      0,
    ) +
    schedule.cuts.reduce((total, c) => total + c.frames * 2 * 4, 0) +
    largestCutFrames * 2 * 4 +
    (schedule.cuts.length
      ? (largestSourceFrames + guardFrames) * 2 * 4 * CLIP_AUDIO_PROCESSING.canonicalRangeCopies
      : 0) +
    CLIP_AUDIO_PROCESSING.dspWorkingBytes +
    CLIP_AUDIO_PROCESSING.encodedPacketBytes
  if (!Number.isSafeInteger(reservedBytes) || reservedBytes > CLIP_BROWSER_RENDER.audioDecodedBytes)
    throw new BrowserAudioRenderError('memory')
  const rangePcmBytes = Math.floor(
    (CLIP_BROWSER_RENDER.audioDecodedBytes - reservedBytes) /
      CLIP_AUDIO_PROCESSING.transientRangeCopies,
  )
  if (schedule.cuts.length && rangePcmBytes <= 0) throw new BrowserAudioRenderError('memory')
  return { schedule, reservedBytes, rangePcmBytes }
}
