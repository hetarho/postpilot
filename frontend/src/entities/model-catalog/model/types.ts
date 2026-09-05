import i18next from 'i18next'
import { MODEL_PURPOSES, type ModelPurpose } from '@/shared/config'

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
  /** The stages this model is registered to serve (change 20). Each stage's picker lists
   *  exactly its members — fitness is never re-derived from capability flags here. */
  stages: readonly StageName[]
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

/** The reasoning override an operator may set for ONE (model, purpose). `''` defers to the
 *  stage policy; `unset` deliberately omits the wire key and keeps the provider's own
 *  behavior. */
export type ReasoningEffortName =
  '' | 'unset' | 'none' | 'minimal' | 'low' | 'medium' | 'high' | 'xhigh' | 'max'

/** The full effort vocabulary. Since change 27 it is the **fallback** for a model whose
 *  accepted values the source does not publish — not the whole truth. A model that does
 *  publish a list is offered exactly that list. */
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
  /** A catalog row exists, so `purposes` and `reasoningEffort` are decisions somebody made
   *  rather than the values an un-curated candidate is shown with. */
  curated: boolean
  /** The purposes this model is registered to — the admin tabs' checkbox state. */
  purposes: readonly ModelPurpose[]
  /** What the model can produce; the image/video generation tabs gate on these the way
   *  photo-analysis gates on `vision`. */
  imageOutput: boolean
  videoOutput: boolean
  /** The provider still offered this model at the last successful read. False is a flag for
   *  the operator, never an action: the model is served disabled-with-reason to users. */
  listed: boolean
  /** The override for the PURPOSE this listing was read for, not for the model. The same
   *  model shows its own value on every tab (change 24). */
  reasoningEffort: ReasoningEffortName
  /** What this model recently spent its completion budget on at the listed purpose's stage,
   *  or undefined when nothing has been recorded — which renders as nothing rather than as a
   *  zero that would read as a measurement. */
  reasoningSpend?: ReasoningSpend
  /** What the source publishes about this model's reasoning (change 27). Every falsy value
   *  here means **unknown**, never "supports nothing" — the same rule an unpublished price
   *  follows. */
  reasoning: ReasoningCapability
  /** Upstream publication time in epoch seconds; orders a vendor's models newest-first. */
  sourceCreatedAt: bigint
}

/** What the source says one model accepts for reasoning. It is what turns the effort control
 *  from "the same eight values for every model" into the model's own list. */
export interface ReasoningCapability {
  /** The model reasons at all. False means the effort control is absent, not empty. */
  reasons: boolean
  /** The accepted values, verbatim and in the source's descending order — the order a
   *  selector should offer them in. Empty means the source publishes no list for this model,
   *  and `REASONING_EFFORTS` is what the control offers instead. */
  efforts: readonly ReasoningEffortName[]
  /** What the model uses when reasoning is on and no effort is sent, so `unset` can be
   *  labelled with what it actually means. Empty when unpublished. */
  defaultEffort: ReasoningEffortName | ''
  /** Reasoning cannot be turned off: `none` is never offered. */
  mandatory: boolean
  /** The provider takes the effort string itself rather than a budget derived from it.
   *  Nothing renders it yet; change 29 consumes it. */
  nativeEffort: boolean
  /** The source offers a reasoning token budget for this model. Recorded and displayed
   *  only — this change surfaces no input for it. */
  maxTokens: boolean
  /** The stored override would be refused if it were written today — no longer in the
   *  published list, `none` on a model that became mandatory, an effort on a model that
   *  stopped reasoning. A warning for the row and nothing more: the value is kept and still
   *  sent. */
  drifted: boolean
  /** The fields above came from a read that actually looked. It is the one thing they cannot
   *  say about themselves: `reasons: false` with no list is both "the source publishes no
   *  reasoning object" and "nothing has asked yet" — every row predating this data reads that
   *  way. False for an entry served from storage (before the first successful refresh, or
   *  while the provider catalog cannot be read), and the control must then keep offering the
   *  full vocabulary rather than disappear while its stored value is still being sent. */
  known: boolean
}

/** A recent window of one model's completion budget at one stage. It is the only reliable
 *  check that a model honors its effort: the provider says a model ACCEPTS
 *  `reasoning_effort`, never which values it honors, and an unhonored effort behaves like
 *  sending none — reasoning then runs to the cap. */
export interface ReasoningSpend {
  calls: bigint
  reasoningTokens: bigint
  completionTokens: bigint
}

/** The share of the completion budget spent on reasoning, 0–1. Zero completion tokens
 *  reports 0 rather than dividing. */
export function reasoningShare(spend: ReasoningSpend): number {
  if (spend.completionTokens <= 0n) return 0
  return Number(spend.reasoningTokens) / Number(spend.completionTokens)
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

export function isModelPurpose(value: string): value is ModelPurpose {
  return (MODEL_PURPOSES as readonly string[]).includes(value)
}

/** A stage lists exactly the models registered to its purpose (change 20) — observe's old
 *  vision-only rule is subsumed, because photo-analysis registration already requires
 *  vision. Disabled models stay in the list — greyed, with the reason — rather than
 *  vanishing, so the user learns why a model is unavailable. */
export function filterForStage(models: readonly CatalogModel[], stage: StageName): CatalogModel[] {
  return models.filter((model) => model.stages.includes(stage))
}
