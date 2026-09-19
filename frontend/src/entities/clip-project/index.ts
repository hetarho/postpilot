/** The clip project itself: its lifecycle, its sources, its price and the state it is in. */
export {
  clipProjectsKey,
  toClipProject,
  toClipSourceBatch,
  useClipProject,
  useClipProjectMutations,
  useClipProjects,
} from './api/clip-project'
export {
  compositionInputsToProto,
  toProjectComposition,
  useClipCapabilities,
} from './api/composition'
export { toClipQuote, toClipRevisionQuote } from './api/credits'
export { useClipAnalysisEligibility } from './api/eligibility'
export { useClipLifecycleApi } from './api/lifecycle'
export {
  getClipSourcePlayback,
  getClipSources,
  setClipSourceOriginalSound,
  useReorderClipSources,
} from './api/sources'
export { boundedText } from './lib/bounded-text'
export type { ClipSourceAssociation } from './model/composition'
export {
  emptyCompositionInputs,
  matchingCompositionInputs,
  projectCompositionDocument,
} from './model/composition-inputs'
export { clipEligibilityOf } from './model/eligibility'
export type { ClipEligibilityStatus, ClipModelEligibility } from './model/eligibility'
export { clipNoticeKey } from './model/notices'
export type { ClipNotice } from './model/notices'
export { preferredClipRenderKind } from './model/render-kind'
export { clipState, clipStateLabel } from './model/state'
export type { ClipState } from './model/state'
export {
  CLIP_PROJECT_LIMITS,
  CLIP_RATIOS,
  emptyClipProject,
  normalizeClipProject,
  projectDraft,
  savableClipProject,
  validClipProject,
  validNewClipProject,
} from './model/types'
export type {
  ClipAccounting,
  ClipProject,
  ClipProjectDraft,
  ClipQuote,
  ClipRatio,
  ClipRenderKind,
  ClipSourceAvailability,
  ClipSourceBatch,
  ClipSourceMetadata,
  ReadyClipBatch,
} from './model/types'
export { ClipCompositionInputFields } from './ui/ClipCompositionInputs'
export { ClipFailureNotice } from './ui/ClipFailureNotice'
export { ClipNoticeList } from './ui/ClipNoticeList'
export { ClipQuoteApproval } from './ui/ClipQuoteApproval'
export { ClipRequestRecord } from './ui/ClipRequestRecord'
export { ClipSourceStrip } from './ui/ClipSourceStrip'
