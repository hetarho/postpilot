import { CLIP_BROWSER_RENDER, CLIP_DESIGN } from '@/entities/clip-design/@x/clip-preview'
import { cutRate, timelineCuts, type ClipEditPlan } from '@/entities/clip-plan/@x/clip-preview'
import { clipSourceSound, textInterval } from '@/entities/clip-plan/@x/clip-preview'
import { spokenState } from '@/entities/clip-plan/@x/clip-preview'

/** Source state, edit clock and audio seams are shared with the server's composition graph. */
export function browserAudioPlan(plan: ClipEditPlan) {
  const timeline = timelineCuts(plan)
  const durationMs = timeline.at(-1)?.endMs ?? 0
  const narration = plan.narration?.enabled ? plan.narration : undefined
  const speechIssues =
    narration?.segments.flatMap((s) => {
      const state = spokenState(narration, s, durationMs)
      return state === 'ready' ? [] : [{ segmentId: s.id, state, previous: !!s.speech }]
    }) ?? []
  const speech =
    narration?.segments.flatMap((s) =>
      spokenState(narration, s, durationMs) === 'ready' && s.speech
        ? [
            {
              segmentId: s.id,
              start: s.startMs / 1000,
              duration: s.speech.samples / s.speech.sampleRate,
              speech: s.speech,
            },
          ]
        : [],
    ) ?? []
  const sampleRate = CLIP_BROWSER_RENDER.audioSampleRate
  const videoFrames = Math.round(
    ((timeline.at(-1)?.endMs ?? 0) * CLIP_BROWSER_RENDER.frameRate) / 1000,
  )
  const cuts = timeline
    .filter((item) => clipSourceSound(plan, item.cut))
    .map((item) => {
      const next = timeline[item.index + 1]
      return {
        cutId: item.cut.id,
        fingerprint: item.cut.fingerprint,
        sourceStart: Math.round((item.cut.startMs * sampleRate) / 1000),
        sourceFrames: Math.round(((item.cut.endMs - item.cut.startMs) * sampleRate) / 1000),
        frames: Math.round(((item.endMs - item.startMs) * sampleRate) / 1000),
        start: item.startMs / 1000,
        end: item.endMs / 1000,
        rate: cutRate(item.cut) / 1000,
        volume: (item.cut.volumePermille / 1000) * ((plan.sourceVolumePermille ?? 1000) / 1000),
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
  hooks.sort((a, b) => a.start - b.start)
  const dip: { start: number; end: number }[] = []
  for (const window of hooks) {
    const previous = dip.at(-1)
    if (previous && window.start <= previous.end) previous.end = Math.max(previous.end, window.end)
    else dip.push({ ...window })
  }
  return {
    cuts,
    speech,
    speechIssues,
    narrationVolume: (narration?.volumePermille ?? 1000) / 1000,
    sourceVolume: (plan.sourceVolumePermille ?? 1000) / 1000,
    dip,
    sampleRate,
    sampleFrames: (videoFrames * sampleRate) / CLIP_BROWSER_RENDER.frameRate,
  }
}

/** The native source adapter keeps pitch; this envelope follows its output-time boundaries. */
export function previewSourceEnvelope(
  schedule: ReturnType<typeof browserAudioPlan>,
  cutId: string,
  time: number,
) {
  const cut = schedule.cuts.find((cut) => cut.cutId === cutId)
  if (!cut || time < cut.start || time >= cut.end) return 0
  const fadeIn = cut.fadeIn ? Math.min(1, (time - cut.start) / cut.fadeIn) : 1
  const fadeOut = cut.fadeOut ? Math.min(1, (cut.end - time) / cut.fadeOut) : 1
  const dip = schedule.dip.some((dip) => time >= dip.start && time < dip.end)
    ? 10 ** (CLIP_DESIGN.audio.hook_dip_db / 20)
    : 1
  return schedule.sourceVolume * Math.max(0, Math.min(fadeIn, fadeOut)) * dip
}
