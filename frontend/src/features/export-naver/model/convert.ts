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

/** The `IMAGE` blocks' filenames in the exact order their `사진_<n>_…_사진` markers appear in
 *  `toNaver`'s output — one entry per marker, always, so position n-1 here is marker n there.
 *
 *  It walks the same canonical block array `toNaver` does, so the photo strip beside the text and
 *  the markers inside it cannot drift: a photo on screen matches a marker in the pasted text by
 *  position, not by counting.
 *
 *  An `IMAGE` block with an EMPTY file is kept, as an empty string. `toNaver` still spends a
 *  number on it, so dropping it here would shift every later photo against its marker — the one
 *  thing this function exists to prevent. The strip renders it as a marker with no photo behind
 *  it.
 *
 *  Duplicates are kept as-is. A filename is unique within a post, so two markers for one file
 *  would be two markers in the text too, and the strip must say the same thing the text says. */
export function naverPhotoOrder(content: Pick<PostContent, 'blocks'>): string[] {
  return walkBlocks(content, (block) =>
    block.type === BlockType.IMAGE ? block.file : null,
  ).filter((file): file is string => file !== null)
}

/** One photo's marker: the word, the number, the folded caption, the word again. A block with
 *  no caption — or one whose caption folds to nothing — is the bare `사진_<n>_사진`, which is
 *  what the marker was before captions rode in it. */
function photoMarker(contentLanguage: ContentLanguage, number: number, caption: string): string {
  const word = contentLanguage === 'en' ? 'photo' : '사진'
  const folded = foldCaption(caption)
  return folded === '' ? `${word}_${number}_${word}` : `${word}_${number}_${folded}_${word}`
}

/** The caption as one `_`-joined token (EXPORT-5).
 *
 *  Every run of whitespace, punctuation and symbols becomes a single `_` and nothing else is
 *  touched: a double-click selects a word, and a space or a comma inside the marker is exactly
 *  where that selection would stop. `맥북(M4)` folds to `맥북_M4`; a caption of nothing but
 *  emoji folds to the empty string, which is why the caller falls back to the bare marker.
 *
 *  The Unicode classes need the `u` flag. The build targets modern browsers only (ARCH-12), so
 *  there is no polyfill and no hand-kept list of punctuation to fall out of date. */
function foldCaption(caption: string): string {
  return caption
    .trim()
    .replace(/[\s\p{P}\p{S}]+/gu, '_')
    .replace(/^_+|_+$/g, '')
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
  // Marker numbers run from 1 in marker order over this one walk, which is the same order
  // `naverPhotoOrder` reports and the same order the preview renders.
  let markerNumber = 0
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
        // A number and the caption, and no filename (EXPORT-5). The marker is a POSITION the
        // author replaces with the photo itself; the number alone could not say WHICH photo
        // that position was for, so the caption rides inside the marker as a label — folded,
        // so the whole thing is one double-click selection. The caption still has its own
        // copy control for the platform's caption box, because this one goes away with the
        // marker (EXPORT-24). No brackets either, which is what tells it from an unfilled
        // template slot (EXPORT-4). A block with an empty `file` still spends its number: the
        // numbering and `naverPhotoOrder` agree by position, so a hole here would shift every
        // later photo against its marker.
        return photoMarker(contentLanguage, ++markerNumber, block.caption)
      case BlockType.VIDEO:
        // A marker, never a URL and never bytes: the clipboard cannot carry a video file from a
        // page, and the file the author filmed is on the device they are pasting from
        // (VIDEO-14, VIDEO-15). It keeps its brackets, its filename AND its caption: a clip has
        // no copy control of its own to carry either one, so the marker is the only place they
        // can travel (EXPORT-5).
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
