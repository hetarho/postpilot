import i18next from 'i18next'
import type { PlanName } from '@/entities/plan/@x/model-catalog'

/** The three places a model is chosen ([I3]); the app never fills one in. */
export type StageName = 'observe' | 'write' | 'analyze'

export const STAGES: readonly StageName[] = ['observe', 'write', 'analyze']

export function stageLabel(stage: StageName): string {
  return i18next.t(`stage.${stage}`, { ns: 'models' })
}

/** One model of one provider — what a job records and what a dropdown saves. */
export interface ModelRef {
  providerId: string
  modelId: string
}

/** A registry entry as the catalog exposes it: ids, label and flags. Never a key. */
export interface CatalogModel {
  ref: ModelRef
  label: string
  vision: boolean
  structuredOutput: boolean
  disabled: boolean
  disabledReason: string
  contextTokens: bigint
  inputUsdPerMillion: string
  outputUsdPerMillion: string
  pricingCheckedAt: string
  /** The lowest plan allowed to run this model, declared per model in the registry. */
  minPlan: PlanName | undefined
  /** `minPlan` is above the CALLING account's tier. Display only: the server refuses a
   *  locked ref on every RPC that accepts one, whatever this client rendered. */
  locked: boolean
}

export type SelectionSlotName = 'active' | 'candidateA' | 'candidateB'

/** The acting user's saved choice for a stage. `missing`: the model is no longer
 *  registered — the server has already cleared the row; this is shown once. */
export interface StageSelection {
  stage: StageName
  ref: ModelRef
  missing: boolean
  slot: SelectionSlotName
}

export interface ComparisonPair {
  stage: StageName
  candidateA?: StageSelection
  candidateB?: StageSelection
}

export interface RecommendationStageSelection {
  stage: StageName
  active: ModelRef
  candidateA: ModelRef
  candidateB: ModelRef
}

export interface RecommendationSet {
  id: string
  label: string
  selections: RecommendationStageSelection[]
}

export function refKey(ref: ModelRef): string {
  return `${ref.providerId}/${ref.modelId}`
}

export function sameRef(a: ModelRef, b: ModelRef): boolean {
  return a.providerId === b.providerId && a.modelId === b.modelId
}

/** The observe stage looks at photos, so it lists vision models only (PRD §6.4); the
 *  other stages list everything. Disabled models stay in the list — greyed, with the
 *  reason — rather than vanishing, so the user learns why a model is unavailable. */
export function filterForStage(models: readonly CatalogModel[], stage: StageName): CatalogModel[] {
  return stage === 'observe' ? models.filter((model) => model.vision) : [...models]
}
