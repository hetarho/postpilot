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
