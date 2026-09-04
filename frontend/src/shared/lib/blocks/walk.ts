import type { Block, PostContent } from '@/shared/api'

export type BlockVisitor<Result> = (block: Block, index: number) => Result

/** Typed iteration keeps each exporter exhaustive over the same canonical block array. */
export function walkBlocks<Result>(
  content: Pick<PostContent, 'blocks'>,
  visitor: BlockVisitor<Result>,
): Result[] {
  return content.blocks.map(visitor)
}

export function headingTag(level: number): 'h2' | 'h3' {
  return level === 3 ? 'h3' : 'h2'
}

/** The placeholder an unfilled template slot exports as, or null for an ordinary block.
 *
 *  Every one of the four export formats needs the same string, and the four export features
 *  are same-layer siblings that may not import each other (ARCHITECTURE §3.1) — so it lives
 *  here, beside `walkBlocks`, as a pure function over one canonical block.
 *
 *  Bracketed, because a person has to see a position they still have to fill after pasting.
 *  The label is the template author's own words and falls back to the slot's kind, so the
 *  placeholder is never an empty pair of brackets. The `[사진 …]` photo markers keep their
 *  filename suffix, which is what tells the two apart in the pasted body. */
export function blockSlotPlaceholder(block: {
  slot?: { kind: string; label: string }
}): string | null {
  if (!block.slot) return null
  const label = block.slot.label.trim() || block.slot.kind
  return `[${label}]`
}

/** How many positions a rendered post still leaves for a person to fill. */
export function unfilledSlotCount(content: { blocks: readonly { slot?: unknown }[] }): number {
  return content.blocks.filter((block) => Boolean(block.slot)).length
}
