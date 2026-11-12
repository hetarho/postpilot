import {
  cutOutputMs,
  outputToSourceMs,
  textInterval,
  timelineCuts,
  type ClipEditPlan,
  type ClipEditableText,
  type ClipSpokenSegment,
  type TimelineEdit,
} from '@/entities/clip-plan'

export interface SourceBounds {
  startMs: number
  endMs: number
}

/** The scene block an item starting at `outputMs` stands under (CLIP-17): the cut whose
 *  transformed output interval contains it. A start exactly on a boundary — or inside a
 *  transition, where two cuts overlap — belongs to the later cut, and one before the first cut
 *  to the first. -1 only when there is no cut at all. */
export function sceneBlockOf(
  outputMs: number,
  cuts: ReadonlyArray<{ startMs: number; endMs: number }>,
): number {
  let index = cuts.length ? 0 : -1
  for (let i = 0; i < cuts.length; i++) if (cuts[i].startMs <= outputMs) index = i
  return index
}

export interface ClipSceneBlock {
  cut: ReturnType<typeof timelineCuts>[number]
  captions: ClipEditableText[]
  spoken: ClipSpokenSegment[]
}

/** One block per cut in output order, each holding the captions and spoken segments whose start
 *  lies inside it, in start order. Region and badge elements are not captions and stand in
 *  blocks of their own. */
export function clipSceneBlocks(plan: ClipEditPlan): ClipSceneBlock[] {
  const cuts = timelineCuts(plan)
  const blocks: ClipSceneBlock[] = cuts.map((cut) => ({ cut, captions: [], spoken: [] }))
  const start = (text: ClipEditableText) => textInterval(plan, text).startMs
  for (const text of plan.elements ?? []) {
    if (text.role !== 'caption') continue
    const index = sceneBlockOf(start(text), cuts)
    if (index >= 0) blocks[index].captions.push(text)
  }
  for (const segment of plan.narration?.segments ?? []) {
    const index = sceneBlockOf(segment.startMs, cuts)
    if (index >= 0) blocks[index].spoken.push(segment)
  }
  for (const block of blocks) {
    block.captions.sort((a, b) => start(a) - start(b))
    block.spoken.sort((a, b) => a.startMs - b.startMs)
  }
  return blocks
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
