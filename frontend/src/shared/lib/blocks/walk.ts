import { BlockType, type Block, type PostContent } from '@/shared/api'

export type BlockVisitor<Result> = (block: Block, index: number) => Result

/** Typed iteration keeps each exporter exhaustive over the same canonical block array. */
export function walkBlocks<Result>(
  content: Pick<PostContent, 'blocks'>,
  visitor: BlockVisitor<Result>,
): Result[] {
  return content.blocks.map(visitor)
}

/** The attached photos a block names, in the order they stand: an IMAGE block's one file (kept
 *  when empty, because the Naver markers still spend a number on it), a photo group's files, and
 *  none for any other block. Every surface that counts or numbers photos reads them through this
 *  one function, so a group can never be counted as one photo in one place and three in another. */
export function blockPhotos(block: Block): readonly string[] {
  switch (block.type) {
    case BlockType.IMAGE:
      return [block.file]
    case BlockType.GALLERY:
      return block.files
    default:
      return []
  }
}

export function headingTag(level: number): 'h2' | 'h3' {
  return level === 3 ? 'h3' : 'h2'
}
