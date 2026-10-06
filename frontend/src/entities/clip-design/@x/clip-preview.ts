/** What `clip-design` exposes to `clip-preview` (ARCH-13 @x). */
export { CLIP_BROWSER_RENDER } from '../config/clip-browser-render'
export { CLIP_COMPOSITION_LIMITS, CLIP_DRAFT_PREVIEW } from '../config/clip-composition'
export { CLIP_DESIGN, CLIP_TRANSITION } from '../config/clip-design'
export {
  CLIP_BROWSER_COMPOSITION,
  CLIP_BROWSER_FONTS,
  CLIP_BROWSER_STATIC_CAPTIONS,
} from '../config/browser-composition'
export type { ClipRatioId } from '../config/clip-design'
export {
  CLIP_INK,
  CLIP_INK_FONT_DATA,
  CLIP_INK_IDENTITY,
  CLIP_CAPTION_INK,
  CLIP_CAPTION_TRANSFORM_PAINT,
  CLIP_CAPTION_EFFECT_PAINT,
  CLIP_CAPTION_EFFECT_STYLES,
  CLIP_CAPTION_TRANSFORM_STYLES,
  clipInkFont,
  clipInkCoverage,
  clipInkMetrics,
} from '../config/clip-ink'
export type { ClipInkFont, ClipInkFace, ClipCaptionInkStyle } from '../config/clip-ink'
export {
  clipLayoutRegion,
  clipRegionPreset,
  clipRegionSlots,
  clipTextWidth,
} from '../model/region-layout'
export type {
  ClipRegionLayout,
  ClipRegionShape,
  ClipRegionSlotSpec,
  ClipPlacedRegionSlot,
} from '../model/region-layout'
