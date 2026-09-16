/** Where a tile dropped at `x` belongs (CLIP-136, CLIP-55).
 *
 *  The strip lays its tiles out horizontally and scrolls, so the target is the
 *  tile whose own box the pointer is over — measured on the tiles themselves
 *  rather than on an assumed width, which a wrapped or scrolled strip would get
 *  wrong. A pointer past either end lands at that end.
 *
 *  It is a pure function of the boxes and the pointer so the drag path and the
 *  keyboard path share one meaning of "where this goes". */
export function reorderTargetIndex(
  boxes: readonly { left: number; right: number }[],
  x: number,
): number {
  if (boxes.length === 0) return 0
  for (let i = 0; i < boxes.length; i++) {
    const box = boxes[i]
    if (x < box.right) return i < 0 ? 0 : i
  }
  return boxes.length - 1
}

/** The same list with one entry moved to another index. Out-of-range targets
 *  clamp, so "move earlier" on the first tile is simply a no-op. */
export function moveInOrder<T>(values: readonly T[], from: number, to: number): T[] {
  const out = [...values]
  if (from < 0 || from >= out.length) return out
  const target = Math.max(0, Math.min(out.length - 1, to))
  const [moved] = out.splice(from, 1)
  out.splice(target, 0, moved)
  return out
}
