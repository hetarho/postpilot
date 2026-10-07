/** What `clip-plan` exposes to `clip-preview` (ARCH-13 @x). */
export { clipPlanToProto, toClipEditingState } from '../api/edit-plan'
export {
  cutOutputMs,
  cutRate,
  outputToSourceMs,
  sourceAudioEnabled,
  sourceToOutputMs,
  timelineCuts,
  copyClipPlan,
} from '../model/edit-plan'
export type {
  ClipEditPlan,
  ClipEditableText,
  ClipTimelineCut,
  RetainedClipSource,
} from '../model/edit-plan'
export { clipSourceSound, textInterval } from '../model/timeline'
export { speechDurationMs, spokenState } from '../model/spoken'
export type { ClipSpeechRef } from '../model/spoken'
export { splitRapid, copyChars } from '../model/caption-pace'
export type {
  ClipCaptionFragment,
  ClipCaptionPreview,
  ClipRegionPresetSamples,
  ClipCanvasBox,
} from '../api/caption-preview'
