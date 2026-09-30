import { orderModelsForStage, type LevelName } from './level'
import i18next from 'i18next'
import { MODEL_PURPOSES } from '../config'
import { type ModelPurpose } from '../config'
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
  /** The model takes VIDEO input. Narrower than `vision` and checked per RUN, not per stage:
   *  a video-blind model still serves every post without a clip (VIDEO-11). */
  videoInput: boolean
  /** Derived transport readiness. Missing (older snapshots) means unsupported. */
  signedVideoUrl?: boolean
  inlineStaticVideo?: boolean
  structuredOutput: boolean
  /** The stages this model is registered to serve (MODEL-14). Each stage's picker lists
   *  exactly its members — fitness is never re-derived from capability flags here. */
  stages: readonly StageName[]
  /** The operator's grade PER STAGE (MODEL-57), for the stages that have one. A stage
   *  absent from this map is ungraded, which is what every registration starts as. */
  levels: Partial<Record<StageName, LevelName>>
  /** Server-owned plan and free-route decision for each listed stage. */
  access?: Partial<Record<StageName, ModelStageAccess>>
  disabled: boolean
  disabledReason: string
  contextTokens: bigint
  /** What one job using this model would hold, for the CALLING account. */
  requiredCredits: number
  /** The caller's balance covers `requiredCredits`. Display only: the server refuses an
   *  unaffordable start whatever this client rendered — and unlike the plan floor it
   *  replaces, it is temporary, so nothing treats an unaffordable choice as invalid. */
  affordable: boolean
  /** The server could not price one call — no eligible official rate (QUOTA-59) or no
   *  bounded model price. As temporary as `affordable`, and it replaces that reason: the
   *  zero `requiredCredits` it comes with would otherwise read as "costs 0". */
  priceUnavailable?: boolean
}

export interface ModelStageAccess {
  grade: LevelName
  requiredPlan: string
  entitled: boolean
  freePathAvailable: boolean
  unavailableReason: string
}

export type SelectionSlotName = 'active' | 'candidateA' | 'candidateB'

/** The acting user's saved choice for a stage. `missing`: the model is no longer
 *  registered — the server has already cleared the row; this is shown once. */
export interface StageSelection {
  stage: StageName
  ref: ModelRef
  missing: boolean
  slot: SelectionSlotName
  requiredPlan?: string
  unavailableReason?: string
}

export interface ComparisonPair {
  stage: StageName
  candidateA?: StageSelection
  candidateB?: StageSelection
}

/** One stage of a recommendation set. Analyze keeps its active selection alone, so its
 *  candidates are absent; observe and write carry their A/B pair (MODEL-23). */
export interface RecommendationStageSelection {
  stage: StageName
  active: ModelRef
  candidateA?: ModelRef
  candidateB?: ModelRef
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

/** The full effort vocabulary. Under MODEL-21 it is the **fallback** for a model whose
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
  videoInput: boolean
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
   *  model shows its own value on every tab (MODEL-7). */
  reasoningEffort: ReasoningEffortName
  /** The operator's grade for the PURPOSE this listing was read for (MODEL-57), '' while
   *  they have not set one. Per registration for the same reason the effort is: the same
   *  model is a different bargain for an input-heavy task than for an output-heavy one. */
  level: LevelName | ''
  /** What this model recently spent its completion budget on at the listed purpose's stage,
   *  or undefined when nothing has been recorded — which renders as nothing rather than as a
   *  zero that would read as a measurement. */
  reasoningSpend?: ReasoningSpend
  /** What the source publishes about this model's reasoning (MODEL-21). Every falsy value
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
   *  Nothing renders it; the server's budget headroom reads it (MODEL-46). */
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
  reasoningTruncations: bigint
}

/** The share of the completion budget spent on reasoning, 0–1. Zero completion tokens
 *  reports 0 rather than dividing. */
export function reasoningShare(spend: ReasoningSpend): number {
  if (spend.completionTokens <= 0n) return 0
  return Number(spend.reasoningTokens) / Number(spend.completionTokens)
}

/** One read of the operator's catalog, with everything the screen has to say about where it
 *  came from. `fetchError` set means the entries are curated rows only. */
/** One estimator combo's current assignment, for the operator's screen. Empty ids are the
 *  never-assigned state a comparison shows no estimate for (QUOTA-39). */
export interface EstimatorComboAssignment {
  combo: string
  observeModelId: string
  writeModelId: string
}

export interface CatalogBrowse {
  entries: readonly AdminCatalogEntry[]
  fetchedAt: string
  fromCache: boolean
  fetchError: string
  /** All four combos in ladder order, assigned or not, so the operator sees which price
   *  tier is still missing a model. */
  estimatorCombos: readonly EstimatorComboAssignment[]
}

/** One line the server refused, with the slug it refused it for. The operator surface maps the
 *  slug to its own copy: these are master-only admin detail, not one of the normalized
 *  user-facing failure reasons. */
export interface CatalogDocumentIssue {
  line: number
  text: string
  cause: string
}

/** What applying a pasted document would do to one purpose. A purpose the document gives no
 *  section for is ABSENT from the plan — untouched, which is not an empty diff. */
export interface CatalogDocumentPurposePlan {
  purpose: ModelPurpose
  register: string[]
  deregister: string[]
  unchanged: string[]
  /** The registrations the document KEEPS but re-grades (MODEL-59). Its own list because a
   *  curator's paste with no grades clears every one of them, and that has to be readable
   *  before 확정 rather than hidden inside "already in place". */
  relevel: CatalogDocumentLevelChange[]
}

/** One registration's grade moving. Either side is '' when it is unset — that is what
 *  setting a first grade and clearing one look like. */
export interface CatalogDocumentLevelChange {
  modelId: string
  from: LevelName | ''
  to: LevelName | ''
}

/** The answer to both preview and apply: they report identically, so a rejection the catalog
 *  moved into between the two renders exactly like one caught at preview. */
export interface CatalogDocumentPlan {
  purposes: CatalogDocumentPurposePlan[]
  issues: CatalogDocumentIssue[]
  fetchError: string
  applied: boolean
  /** What the document does to the recommendation sets, by label (MODEL-72). Undefined when it
   *  has no `[recommendations]` section — the sets are untouched, which is not the same as a
   *  section that changes nothing. */
  recommendations?: CatalogDocumentRecommendationPlan
}

export interface CatalogDocumentRecommendationPlan {
  added: string[]
  removed: string[]
  /** Same label, different models. */
  changed: string[]
  unchanged: string[]
  /** The kept sets come in a different order than they are stored in. */
  reordered: boolean
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

/** The purpose each user-facing stage is fed by (MODEL-14). */
export const STAGE_PURPOSE: Readonly<Record<StageName, ModelPurpose>> = {
  observe: 'photo-analysis',
  analyze: 'style-analysis',
  write: 'writing',
}

/** A recommendation set's stages in the order its seven slots are shown and saved. */
export const RECOMMENDATION_STAGES: readonly StageName[] = ['observe', 'analyze', 'write']

/** The slots a set fills for a stage: the active model, plus the A/B pair where the stage keeps
 *  one (MODEL-23). */
export function recommendationSlots(stage: StageName): readonly SelectionSlotName[] {
  return stage === 'analyze' ? ['active'] : ['active', 'candidateA', 'candidateB']
}

/** The wire key a refused draft names a field by (MODEL-70): `label`, or `<stage>_<slot>`. */
export function recommendationField(stage: StageName, slot: SelectionSlotName): string {
  const wireSlot = { active: 'active', candidateA: 'candidate_a', candidateB: 'candidate_b' }[slot]
  return `${stage}_${wireSlot}`
}

/** Why the server refused one field of a draft set (MODEL-70). */
export type RecommendationFieldCause =
  'required' | 'too_long' | 'duplicate' | 'unregistered' | 'unclassified'

export function isRecommendationFieldCause(value: string): value is RecommendationFieldCause {
  return ['required', 'too_long', 'duplicate', 'unregistered', 'unclassified'].includes(value)
}

/** A stage lists exactly the models registered to its purpose (MODEL-14) — observe's old
 *  vision-only rule is subsumed, because photo-analysis registration already requires
 *  vision. Disabled models stay in the list — greyed, with the reason — rather than
 *  vanishing, so the user learns why a model is unavailable.
 *
 *  The result is ORDERED 가성비 → 최고 with ungraded models last (MODEL-44). Ordering here
 *  rather than at each call site is deliberate: three fields across two features ask this
 *  question, and one of them forgetting to sort would look like a catalog bug. */
export function filterForStage(models: readonly CatalogModel[], stage: StageName): CatalogModel[] {
  return orderModelsForStage(
    models.filter(
      (model) => model.stages.includes(stage) && (!model.access || Boolean(model.access[stage])),
    ),
    stage,
  )
}
