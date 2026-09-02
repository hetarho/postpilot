import type { PostImage } from '@/entities/image'
import { BlockType, type ContentLanguage, type PostContent } from '@/shared/api'
import { walkBlocks } from '@/shared/lib'

/** The `IMAGE` blocks' filenames in the exact order their `[사진 …]` markers appear in
 *  `toNaver`'s output — one entry per marker, always.
 *
 *  It walks the same canonical block array `toNaver` does, so the photo strip beside the text and
 *  the markers inside it cannot drift: a photo on screen matches a marker in the pasted text by
 *  position, not by counting.
 *
 *  An `IMAGE` block with an EMPTY file is kept, as an empty string. `toNaver` still writes a
 *  `[사진 ]` marker for it, so dropping it here would shift every later photo against its marker
 *  — the one thing this function exists to prevent. The strip renders it as a marker with no
 *  photo behind it.
 *
 *  Duplicates are kept as-is. A filename is unique within a post, so two markers for one file
 *  would be two markers in the text too, and the strip must say the same thing the text says. */
export function naverPhotoOrder(content: Pick<PostContent, 'blocks'>): string[] {
  return walkBlocks(content, (block) =>
    block.type === BlockType.IMAGE ? block.file : null,
  ).filter((file): file is string => file !== null)
}

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
