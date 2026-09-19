/** What the clip-project entity exposes to clip-template (ARCH-13 @x): the clip
 *  design system and the composition grammar's bounds, which the template
 *  builder validates and previews against. */
export {
  CLIP_DESIGN,
  CLIP_REGIONS,
  CLIP_RULES,
  CLIP_PRESETS,
  CLIP_DEFAULT_REGION_PRESETS,
  CLIP_COMPOSITION_LIMITS,
  CLIP_COMPOSITION_PREVIEW,
  CLIP_TIMELINE,
  clipRegion,
} from '../config'
export type { ClipRatioId, ClipPresetId, ClipRegionPresets, ClipCaptionPace } from '../config'
