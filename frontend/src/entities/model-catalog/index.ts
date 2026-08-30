export type {
  CatalogModel,
  ComparisonPair,
  ModelRef,
  RecommendationSet,
  RecommendationStageSelection,
  SelectionSlotName,
  StageName,
  StageSelection,
} from './model/types'
export { STAGES, STAGE_LABELS, filterForStage, refKey, sameRef } from './model/types'
export { useModels } from './api/useModels'
export type {
  SelectionsByStage,
  StageSelectionState,
  UnavailableSelection,
} from './api/useSelections'
export {
  UNSUITABLE_REASON,
  VANISHED_REASON,
  useSelections,
  useStageSelection,
} from './api/useSelections'
export { useSaveSelection, useSelectionSavePending } from './api/useSaveSelection'
export {
  useApplyRecommendation,
  useComparisonPairSavePending,
  useModelSetup,
  useSaveComparisonPair,
} from './api/useModelSetup'
export { getSelectionsQueryKey } from './api/catalog-mappers'
