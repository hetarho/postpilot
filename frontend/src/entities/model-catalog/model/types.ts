import i18next from 'i18next'

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
  /** What one job using this model would hold, for the CALLING account. */
  requiredCredits: number
  /** The caller's balance covers `requiredCredits`. Display only: the server refuses an
   *  unaffordable start whatever this client rendered — and unlike the plan floor it
   *  replaces, it is temporary, so nothing treats an unaffordable choice as invalid. */
  affordable: boolean
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

/** The strict per-model reasoning override an operator may set. `''` defers to the stage
 *  policy; `unset` deliberately omits the wire key and keeps the provider's own behavior. */
export type ReasoningEffortName =
  '' | 'unset' | 'none' | 'minimal' | 'low' | 'medium' | 'high' | 'xhigh' | 'max'

export const REASONING_EFFORTS: readonly ReasoningEffortName[] = [
  '',
  'unset',
  'none',
  'minimal',
  'low',
  'medium',
  'high',
  'xhigh',
  'max',
]

export function isReasoningEffort(value: string): value is ReasoningEffortName {
  return (REASONING_EFFORTS as readonly string[]).includes(value)
}

/** One row of the OPERATOR's catalog: a model the provider offers, a model this installation
 *  has curated, or both. It carries prices and a description the user-facing `CatalogModel`
 *  has no use for — this is the screen where a model is chosen to exist at all. */
export interface AdminCatalogEntry {
  modelId: string
  /** The vendor segment of the id ("openai" in "openai/gpt-5.6-sol") — what the list groups
   *  and filters by. Not the registry's provider id, which is the same for every row. */
  providerSlug: string
  label: string
  description: string
  vision: boolean
  structuredOutput: boolean
  contextTokens: bigint
  inputUsdPerMillion: string
  outputUsdPerMillion: string
  /** A catalog row exists, so `minPlan` and `reasoningEffort` are decisions somebody made
   *  rather than the values an un-curated candidate is shown with. */
  curated: boolean
  enabled: boolean
  /** The provider still offered this model at the last successful read. False is a flag for
   *  the operator, never an action: the model is served disabled-with-reason to users. */
  listed: boolean
  reasoningEffort: ReasoningEffortName
  /** Upstream publication time in epoch seconds; orders a vendor's models newest-first. */
  sourceCreatedAt: bigint
}

/** One read of the operator's catalog, with everything the screen has to say about where it
 *  came from. `fetchError` set means the entries are curated rows only. */
export interface CatalogBrowse {
  entries: readonly AdminCatalogEntry[]
  fetchedAt: string
  fromCache: boolean
  fetchError: string
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
