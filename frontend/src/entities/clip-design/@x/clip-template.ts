/** What `clip-design` exposes to `clip-template` (ARCH-13 @x). */
export { CLIP_COMPOSITION_LIMITS, CLIP_COMPOSITION_PREVIEW } from '../config/clip-composition'
export {
  CLIP_DEFAULT_REGION_PRESETS,
  CLIP_DESIGN,
  CLIP_PRESETS,
  CLIP_REGIONS,
  CLIP_RULES,
} from '../config/clip-design'
export type {
  ClipCaptionPace,
  ClipIntroPresetId,
  ClipOutroPresetId,
  ClipPresetId,
  ClipRatioId,
  ClipRegionPresets,
} from '../config/clip-design'
export {
  clipLayoutRegion,
  clipRegionSlotAt,
  clipRegionSlotBudget,
  clipRegionSlotType,
  clipRegionSlots,
} from '../model/region-layout'
export type {
  ClipRegionKind,
  ClipRegionLayout,
  ClipRegionRatio,
  ClipRegionSlotSpec,
} from '../model/region-layout'
