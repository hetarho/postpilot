import type { PostImage } from '@/entities/image'
import { BlockType, type ContentLanguage, type PostContent } from '@/shared/api'
import { walkBlocks } from '@/shared/lib'

/** Plain text for SmartEditor ONE. The post title is copied separately by the panel. */
export function toNaver(
  content: PostContent,
  images: readonly PostImage[],
  contentLanguage: ContentLanguage,
): string {
  // The attachment objects carry expiring view URLs; export contracts deliberately use
  // only canonical block filenames, including for an unknown-but-already-validated file.
  void images
  return walkBlocks(content, (block) => {
    switch (block.type) {
      case BlockType.TEXT:
      case BlockType.HEADING:
        return block.content
      case BlockType.IMAGE:
        return block.caption
          ? `[${contentLanguage === 'en' ? 'Photo' : '사진'} ${block.file}: ${block.caption}]`
          : `[${contentLanguage === 'en' ? 'Photo' : '사진'} ${block.file}]`
      case BlockType.QUOTE:
        return `“${block.content}”`
      case BlockType.LIST:
        return block.items.map((item) => `- ${item}`).join('\n')
      default:
        return ''
    }
  })
    .filter(Boolean)
    .join('\n\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}
