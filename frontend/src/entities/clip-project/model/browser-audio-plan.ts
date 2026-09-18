import { CLIP_BROWSER_RENDER, CLIP_DESIGN } from '@/shared/config'
import { cutRate, timelineCuts, type ClipEditPlan } from './edit-plan'
import { clipSourceSound, textInterval } from './timeline'

/** Source state, edit clock and audio seams are shared with the server's composition graph. */
export function browserAudioPlan(plan: ClipEditPlan) {
  const timeline = timelineCuts(plan)
  const sampleRate = CLIP_BROWSER_RENDER.audioSampleRate
  const videoFrames = Math.round(
    ((timeline.at(-1)?.endMs ?? 0) * CLIP_BROWSER_RENDER.frameRate) / 1000,
  )
  const cuts = timeline
    .filter((item) => clipSourceSound(plan, item.cut))
    .map((item) => {
      const next = timeline[item.index + 1]
      return {
        fingerprint: item.cut.fingerprint,
        sourceStart: Math.round((item.cut.startMs * sampleRate) / 1000),
        sourceFrames: Math.round(((item.cut.endMs - item.cut.startMs) * sampleRate) / 1000),
        frames: Math.round(((item.endMs - item.startMs) * sampleRate) / 1000),
        start: item.startMs / 1000,
        end: item.endMs / 1000,
        rate: cutRate(item.cut) / 1000,
        volume: item.cut.volumePermille / 1000,
        fadeIn: item.index ? (item.cut.transitionMs || CLIP_DESIGN.audio.crossfade_ms) / 1000 : 0,
        fadeOut: next ? (next.cut.transitionMs || CLIP_DESIGN.audio.crossfade_ms) / 1000 : 0,
      }
    })
  const hooks = (plan.elements ?? [])
    .filter(
      (element) =>
        element.role === 'hook' &&
        (element.text.trim() || element.rows?.some((row) => row.text.trim())),
    )
    .map((element) => textInterval(plan, element))
    .filter((window) => window.valid)
    .map((window) => ({ start: window.startMs / 1000, end: window.endMs / 1000 }))
  if (!plan.nativeComposition && !plan.elements?.length && plan.hook.trim())
    hooks.push({ start: 0, end: CLIP_DESIGN.timing.intro_default_s })
  hooks.sort((a, b) => a.start - b.start)
  const dip: { start: number; end: number }[] = []
  for (const window of hooks) {
    const previous = dip.at(-1)
    if (previous && window.start <= previous.end) previous.end = Math.max(previous.end, window.end)
    else dip.push({ ...window })
  }
  return {
    cuts,
    dip,
    sampleRate,
    sampleFrames: (videoFrames * sampleRate) / CLIP_BROWSER_RENDER.frameRate,
  }
}
