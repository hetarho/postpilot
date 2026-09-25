/** The clip context's own numbers (ARCH-21). They moved out of `shared/config`
 *  with T258: every one of them names the clip domain and only clip slices read
 *  them, so one file that every slice edits is not where they belong. */

export {
  CLIP_DESIGN,
  CLIP_REGIONS,
  CLIP_DEFAULT_CAPTION_STYLE,
  CLIP_DEFAULT_REGION_PRESETS,
  CLIP_CAPTION_STYLES,
  clipCaptionRule,
  clipCaptionSizes,
  CLIP_RULES,
  clipRegion,
  clipCaption,
  CLIP_PRESETS,
  CLIP_DISCLOSURES,
  CLIP_CTAS,
  CLIP_SHADOW,
  CLIP_SPACING,
  CLIP_TIMING,
  CLIP_TRANSITION,
  CLIP_PLAYBACK,
  CLIP_RATES,
  CLIP_COPY,
  CLIP_RAPID,
  CLIP_FACTS,
  CLIP_ACCENT_HEX,
  CLIP_TYPE,
  CLIP_VOICE,
  clipPaint,
} from './clip-design'
export type {
  ClipCaptionPace,
  ClipIntroPresetId,
  ClipOutroPresetId,
  ClipRegionPresets,
  ClipRatioId,
  ClipPresetId,
  ClipCaptionStyleId,
  ClipDisclosureId,
  ClipCTAId,
} from './clip-design'
export {
  CLIP_COMPOSITION_LIMITS,
  CLIP_COMPOSITION_PREVIEW,
  CLIP_DRAFT_PREVIEW,
  CLIP_REVISION,
  CLIP_TIMELINE,
  CLIP_CAPTION_PLACEMENT,
} from './clip-composition'
export { CLIP_BROWSER_RENDER } from './clip-browser-render'
export {
  CLIP_SOURCE_MAX_COUNT,
  CLIP_SOURCE_MAX_FILENAME_CHARS,
  CLIP_SOURCE_MAX_DURATION_MS,
  CLIP_SOURCE_MAX_FILE_BYTES,
  CLIP_SOURCE_MAX_BATCH_BYTES,
  CLIP_SOURCE_FINGERPRINT_CHUNK_BYTES,
  CLIP_SOURCE_CONTAINERS,
} from './sources'
