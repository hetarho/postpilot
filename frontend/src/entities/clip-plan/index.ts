/** The edit plan: cuts, captions, the timeline they are edited on, and the revision requests
 *  that ask for a new one. */
export {
  useClipCaptionPreview,
  useClipCaptionStyleSamples,
  useClipRegionPresetSamples,
} from './api/caption-preview'
export type {
  ClipCanvasBox,
  ClipCaptionFragment,
  ClipRegionPresetSample,
} from './api/caption-preview'
export { clipPlanToProto, toClipEditingState } from './api/edit-plan'
export {
  CLIP_PLAYBACK_RATES,
  copyClipPlan,
  cutOutputMs,
  cutRate,
  outputToSourceMs,
  ownerCutId,
  requiredClipSources,
  sourceToOutputMs,
  timelineCuts,
} from './model/edit-plan'
export type {
  ClipEditCut,
  ClipEditPlan,
  ClipEditableText,
  ClipEditingState,
  RetainedClipSource,
} from './model/edit-plan'
export {
  clipAICaptionSet,
  clipCaptionSizeRange,
  clipCaptionStyleOf,
  clipOwnerSizeFits,
} from './model/caption-style'
export { CLIP_REVISION_TARGETS } from './model/revision'
export {
  CLIP_REGION_ROLES,
  clipRegionElementId,
  clipRegionRows,
  rebaseClipRegions,
} from './model/region-rebase'
export type { ClipRevisionTarget } from './model/revision'
export {
  acknowledgeClipCuts,
  captionStartCut,
  clipDraftKey,
  clipSeconds,
  clipSourceSound,
  clipTextTracks,
  clipTimelineReducer,
  createClipTimeline,
  narrationSlot,
  snapClipTime,
  splitTextPhrases,
  textInterval,
  timelineLabelFits,
  validateTimelinePlan,
  withSourceSound,
} from './model/timeline'
export type { ClipSelection, TimelineEdit } from './model/timeline'
export { useClipPlanCalls, useClipRevisionQuote } from './api/calls'
export { ClipCaptionStyleSample } from './ui/ClipCaptionStyleSample'
export type { ClipPlanCalls } from './api/calls'

export type {
  ClipNarration,
  ClipSpokenSegment,
  ClipSpeechRef,
  ClipDerivedCaption,
} from './model/spoken'
export { SPOKEN_LIMITS, spokenState, speechDurationMs } from './model/spoken'
