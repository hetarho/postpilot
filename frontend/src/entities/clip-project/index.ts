export {
  CLIP_RATIOS,
  CLIP_DISCLOSURES,
  CLIP_CTAS,
  CLIP_PROJECT_LIMITS,
  emptyClipProject,
  projectDraft,
  normalizeClipProject,
  validClipProject,
} from './model/types'
export { clipState, clipStateLabel } from './model/state'
export { useClipLifecycleApi } from './api/lifecycle'
export type { ClipState } from './model/state'
export type {
  ClipRatio,
  ClipDisclosureId,
  ClipCTAId,
  ClipProjectDraft,
  ClipProject,
  ClipSourceMetadata,
  ClipSourceBatch,
  ReadyClipBatch,
  ClipAccounting,
  ClipQuote,
} from './model/types'
export {
  clipProjectsKey,
  toClipProject,
  toClipSourceBatch,
  useClipProjects,
  useClipProject,
  useClipProjectMutations,
} from './api/clip-project'
export {
  allowsSecondCopy,
  classifyCopy,
  firstCopy,
  CLIP_TRANSITIONS,
  CLIP_TRANSITION_CHOICES,
  COPY_ANCHORS,
  COPY_ALIGNS,
  clipPlanDuration,
  copyClipPlan,
  editClipPlan,
  groundedInAnswers,
  requiredClipSources,
  validateClipPlan,
  withinHookLimits,
} from './model/edit-plan'
export type {
  ClipCaption,
  ClipEditCut,
  ClipEditPlan,
  ClipEditingState,
  RetainedClipSource,
  ClipEdit,
} from './model/edit-plan'
export { toClipEditingState, clipPlanToProto } from './api/edit-plan'
export { toClipQuote, toClipAccounting } from './api/credits'
export {
  CLIP_ELIGIBILITY_STATUSES,
  clipEligibilityOf,
  isClipEligibilityStatus,
} from './model/eligibility'
export type {
  ClipEligibilityStatus,
  ClipIneligibility,
  ClipModelEligibility,
} from './model/eligibility'
export {
  clipEligibilityKey,
  toClipEligibility,
  useClipAnalysisEligibility,
} from './api/eligibility'

export { isRapidCut, canSplitRapid, canAddRapid } from './model/caption-pace'
export { observationCutUsage, observationSummary } from './model/observations'
export type {
  ClipObservedSegment,
  ClipSourceObservation,
  ClipObservations,
} from './model/observations'
export { ClipSourceStrip } from './ui/ClipSourceStrip'
export { ClipCompositionInputFields } from './ui/ClipCompositionInputs'
export {
  emptyCompositionInputs,
  matchingCompositionInputs,
  projectCompositionDocument,
  validCompositionInputs,
} from './model/composition-inputs'

export type {
  ClipCompositionInputs,
  ClipProjectComposition,
  ClipSourceAssociation,
} from './model/composition'
export {
  toProjectComposition,
  compositionInputsToProto,
  useClipCapabilities,
} from './api/composition'

export { getClipSources, getClipSourcePlayback } from './api/sources'
export type { ClipSourceAvailability } from './model/types'

export { ClipDraftPreview } from './ui/ClipDraftPreview'
export type { ClipDisplayedFrame } from './ui/ClipDraftPreview'
export type { ClipEditableText } from './model/edit-plan'
export { previewTimeline, previewFrame, previewElementIDs } from './model/draft-preview'
export {
  timelineCuts,
  textInterval,
  nativeTextErrors,
  validateTimelinePlan,
  applyTimelineEdit,
  clipDraftKey,
  selectedTime,
  snapClipTime,
  clipSeconds,
  createClipTimeline,
  clipTimelineReducer,
  splitTextPhrases,
  clipTextTracks,
} from './model/timeline'
export type {
  ClipSelection,
  TimelineEdit,
  ClipTimelineState,
  ClipTimelineAction,
} from './model/timeline'
