import { CLIP_BROWSER_RENDER, CLIP_DESIGN } from '@/entities/clip-design/@x/clip-preview'
import { cutRate, type ClipEditPlan } from '@/entities/clip-plan/@x/clip-preview'
import { clipSourceSound, textInterval } from '@/entities/clip-plan/@x/clip-preview'
import { spokenState } from '@/entities/clip-plan/@x/clip-preview'
import { frameTimeline } from './draft-preview'

/** Source state, edit clock and audio seams are shared with the server's composition graph. */
export function browserAudioPlan(plan: ClipEditPlan) {
  const timeline = frameTimeline(plan, CLIP_BROWSER_RENDER.frameRate)
  const durationMs = (timeline.total * 1000) / timeline.fps
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
  const videoFrames = timeline.total
  const cuts = timeline.cuts
    .filter((item) => clipSourceSound(plan, item.cut))
    .map((item) => {
      const next = timeline.cuts[item.index + 1]
      const start = item.startFrame / timeline.fps
      const end = (item.startFrame + item.frames) / timeline.fps
      const fadeOut = next ? (next.cut.transitionMs || CLIP_DESIGN.audio.crossfade_ms) / 1000 : 0
      // Native hard-cut afade anchors at integer milliseconds, while its PCM
      // length is exact frames*sampleRate/fps. Keep the fractional tail silent.
      const fadeOutStart = next?.overlap
        ? end - fadeOut
        : start + Math.max(0, Math.floor((item.frames * 1000) / timeline.fps) / 1000 - fadeOut)
      return {
        cutId: item.cut.id,
        fingerprint: item.cut.fingerprint,
        sourceStart: Math.round((item.cut.startMs * sampleRate) / 1000),
        sourceFrames: Math.round(((item.cut.endMs - item.cut.startMs) * sampleRate) / 1000),
        sourceStartUs: Math.round(item.cut.startMs * 1000),
        sourceEndUs: Math.round(item.cut.endMs * 1000),
        frames: (item.frames * sampleRate) / timeline.fps,
        startSample: (item.startFrame * sampleRate) / timeline.fps,
        start,
        end,
        rate: cutRate(item.cut) / 1000,
        volume: (item.cut.volumePermille / 1000) * ((plan.sourceVolumePermille ?? 1000) / 1000),
        fadeIn: item.index ? (item.cut.transitionMs || CLIP_DESIGN.audio.crossfade_ms) / 1000 : 0,
        fadeOut,
        fadeOutStart,
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
  const fadeOut = cut.fadeOut
    ? Math.max(0, Math.min(1, 1 - (time - cut.fadeOutStart) / cut.fadeOut))
    : 1
  const dip = schedule.dip.some((dip) => time >= dip.start && time < dip.end)
    ? 10 ** (CLIP_DESIGN.audio.hook_dip_db / 20)
    : 1
  return schedule.sourceVolume * Math.max(0, Math.min(fadeIn, fadeOut)) * dip
}
