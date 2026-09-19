/** The edit plan: cuts, captions, the timeline they are edited on, and the revision requests
 *  that ask for a new one. */
export { useClipCaptionPreview, useClipCaptionStyleSamples } from './api/caption-preview'
export type { ClipCanvasBox, ClipCaptionFragment } from './api/caption-preview'
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
export { CLIP_REVISION_TARGETS } from './model/revision'
export type { ClipRevisionTarget } from './model/revision'
export {
  acknowledgeClipCuts,
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
