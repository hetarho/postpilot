import type { PostImage } from '@/entities/image'
import { BlockType, type ContentLanguage, type PostContent } from '@/shared/api'
import { blockSlotPlaceholder, walkBlocks } from '@/shared/lib'

/** The `VIDEO` blocks' filenames in the order their `[동영상 …]` markers appear, for the same
 *  reason `naverPhotoOrder` exists: the export tab plays each clip beside its own marker, and
 *  pairing by position is what keeps the two from drifting. */
export function naverVideoOrder(content: Pick<PostContent, 'blocks'>): string[] {
  return walkBlocks(content, (block) =>
    block.type === BlockType.VIDEO ? block.file : null,
  ).filter((file): file is string => file !== null)
}

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
    // An unfilled template slot exports as the position it reserves, never as its copy
    // token: the token is machinery for the model, and what a person needs in the pasted
    // body is a place to fill.
    const slot = blockSlotPlaceholder(block)
    if (slot) return slot
    switch (block.type) {
      case BlockType.TEXT:
      case BlockType.HEADING:
        return block.content
      case BlockType.IMAGE:
        return block.caption
          ? `[${contentLanguage === 'en' ? 'Photo' : '사진'} ${block.file}: ${block.caption}]`
          : `[${contentLanguage === 'en' ? 'Photo' : '사진'} ${block.file}]`
      case BlockType.VIDEO:
        // A marker, never a URL and never bytes: the clipboard cannot carry a video file from a
        // page, and the file the author filmed is on the device they are pasting from
        // (VIDEO-14, VIDEO-15). The dash, not the colon a photo uses, is what tells the two
        // marker families apart at a glance in the pasted body.
        return block.caption
          ? `[${contentLanguage === 'en' ? 'Video' : '동영상'} ${block.file} — ${block.caption}]`
          : `[${contentLanguage === 'en' ? 'Video' : '동영상'} ${block.file}]`
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
