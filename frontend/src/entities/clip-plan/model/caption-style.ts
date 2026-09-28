import {
  CLIP_CAPTION_STYLES,
  CLIP_DEFAULT_CAPTION_STYLE,
  clipCaptionSizes,
} from '@/entities/clip-design/@x/clip-plan'
import type { ClipEditableText } from './edit-plan'

/** The styles a writer may choose for this project (CLIP-142): its own AI set, or the default
 *  style alone where it chose none. The owner is not held to it. */
export const clipAICaptionSet = (styles: readonly string[] = []): readonly string[] =>
  styles.length ? styles : [CLIP_DEFAULT_CAPTION_STYLE]

/** The approved style a caption is drawn in, by the server's own rule (`CaptionStyleOf`): the
 *  owner's choice, then the style its plan names — kept whatever the AI set says — and, for a
 *  caption naming none, the AI set's first entry (CLIP-142, CDS-66). */
export function clipCaptionStyleOf(
  text: Pick<ClipEditableText, 'ownerStyle' | 'style'>,
  aiSet: readonly string[] = [],
): string {
  if (text.ownerStyle) return text.ownerStyle
  if (
    text.style &&
    text.style !== 'auto' &&
    (CLIP_CAPTION_STYLES as readonly string[]).includes(text.style)
  )
    return text.style
  return clipAICaptionSet(aiSet)[0]
}

/** The size range the caption may be set at: its drawn style's role, CDS-3's floor to the size
 *  the role is set at (CDS-82). */
export const clipCaptionSizeRange = (
  text: Pick<ClipEditableText, 'ownerStyle' | 'style'>,
  aiSet: readonly string[] = [],
) => clipCaptionSizes(clipCaptionStyleOf(text, aiSet))

/** Whether the owner's own size fits the style the caption is drawn in. A size the new style
 *  cannot take is kept and reported, never reset (CDS-100). */
export function clipOwnerSizeFits(
  text: Pick<ClipEditableText, 'ownerStyle' | 'style' | 'ownerSizePx'>,
  aiSet: readonly string[] = [],
) {
  if (!text.ownerSizePx) return true
  const { min, max } = clipCaptionSizeRange(text, aiSet)
  return text.ownerSizePx >= min && text.ownerSizePx <= max
}
