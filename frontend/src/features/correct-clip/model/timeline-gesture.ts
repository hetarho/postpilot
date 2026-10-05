import {
  cutRate,
  cutOutputMs,
  outputToSourceMs,
  snapClipTime,
  timelineCuts,
  type ClipEditPlan,
  type TimelineEdit,
} from '@/entities/clip-plan'

export type CutGesture = 'start' | 'end' | 'move'
export interface SourceBounds {
  startMs: number
  endMs: number
}
const clamp = (n: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, n))

/** Geometry uses the scrollable strip, whose client rect includes its scroll offset. */
export function timelinePoint(clientX: number, left: number, width: number, durationMs: number) {
  if (!(width > 0) || !Number.isFinite(clientX)) return 0
  return clamp(snapClipTime(((clientX - left) / width) * durationMs), 0, durationMs)
}

/** One gesture produces one final edit. Owner caption/speech windows are never rewritten. */
export function cutGestureEdit(
  plan: ClipEditPlan,
  id: string,
  intent: CutGesture,
  deltaOutputMs: number,
  pointerOutputMs: number,
  bounds: SourceBounds,
): TimelineEdit | undefined {
  const geometry = timelineCuts(plan)
  const item = geometry.find((c) => c.cut.id === id)
  if (!item || !Number.isFinite(deltaOutputMs)) return
  if (intent === 'move') {
    const to = geometry.findIndex((c) => pointerOutputMs < (c.startMs + c.endMs) / 2)
    return { type: 'move', from: item.index, to: to < 0 ? geometry.length - 1 : to }
  }
  const cut = item.cut
  const minimumOutput =
    Math.max(400, cut.transitionMs + (geometry[item.index + 1]?.cut.transitionMs ?? 0)) + 1
  const minimumSource = Math.ceil((minimumOutput * cutRate(cut)) / 1000)
  const delta = Math.round((snapClipTime(deltaOutputMs) * cutRate(cut)) / 1000)
  if (bounds.endMs - bounds.startMs < minimumSource) return
  const patch =
    intent === 'start'
      ? { startMs: clamp(cut.startMs + delta, bounds.startMs, cut.endMs - minimumSource) }
      : { endMs: clamp(cut.endMs + delta, cut.startMs + minimumSource, bounds.endMs) }
  return { type: 'cut', id, patch }
}

/** Overlap has two source frames; splitting there has no unambiguous output frame. */
export function splitAtOutput(
  plan: ClipEditPlan,
  id: string,
  outputMs: number,
  newId: string,
): TimelineEdit | undefined {
  const active = timelineCuts(plan).filter((c) => c.startMs <= outputMs && outputMs < c.endMs)
  if (active.length !== 1 || active[0].cut.id !== id) return
  const item = active[0],
    cut = item.cut
  const sourceMs = outputToSourceMs(item, outputMs)
  if (
    cut.creation ||
    !plan.nativeComposition ||
    cutOutputMs({ ...cut, endMs: sourceMs }) <= Math.max(400, cut.transitionMs) ||
    cutOutputMs({ ...cut, startMs: sourceMs }) <=
      Math.max(400, timelineCuts(plan)[item.index + 1]?.cut.transitionMs ?? 0)
  )
    return
  return { type: 'splitCut', id, newId, sourceMs }
}
