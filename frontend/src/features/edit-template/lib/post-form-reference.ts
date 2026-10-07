import { BlockType, type PostContent } from '@/shared/api'

/** Explicit owner-selected form reference. Files, captions, tags, summary and account settings
 * never enter this projection; attachment blocks contribute positions only. */
export function postFormReference(content: PostContent): string {
  return [
    content.title,
    ...content.blocks.map((block) => {
      switch (block.type) {
        case BlockType.IMAGE:
          return '[photo position]'
        case BlockType.GALLERY:
          return '[photo group position]'
        case BlockType.VIDEO:
          return '[video position]'
        case BlockType.LIST:
          return block.items.join('\n')
        case BlockType.TEXT:
        case BlockType.HEADING:
        case BlockType.QUOTE:
          return block.content
        case BlockType.BLOCK_TYPE_UNSPECIFIED:
          return ''
      }
    }),
  ].join('\n\n')
}
