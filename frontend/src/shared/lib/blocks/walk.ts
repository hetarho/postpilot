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
