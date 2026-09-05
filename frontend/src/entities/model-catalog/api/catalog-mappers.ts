// Proto ↔ domain for the model catalog. The stage enum crosses here and nowhere else.
import type { Transport } from '@connectrpc/connect'
import { createConnectQueryKey } from '@connectrpc/connect-query'
import {
  type ProtoCatalogEntry,
  type ProtoListCatalogResponse,
  type ProtoModelInfo,
  type ProtoModelRef,
  type ProtoSelection,
  ProviderService,
  SelectionSlot,
  Stage,
  type ProtoComparisonPair,
  type ProtoRecommendationSet,
} from '@/shared/api'
import type {
  AdminCatalogEntry,
  CatalogBrowse,
  CatalogModel,
  ComparisonPair,
  ModelRef,
  RecommendationSet,
  SelectionSlotName,
  StageName,
  StageSelection,
} from '../model/types'
import { isModelPurpose, isReasoningEffort } from '../model/types'

const STAGE_TO_PROTO: Record<StageName, Stage> = {
  observe: Stage.OBSERVE,
  write: Stage.WRITE,
  analyze: Stage.ANALYZE,
}

export function stageToProto(stage: StageName): Stage {
  return STAGE_TO_PROTO[stage]
}

export function stageFromProto(stage: Stage): StageName | undefined {
  for (const [name, wire] of Object.entries(STAGE_TO_PROTO) as [StageName, Stage][]) {
    if (wire === stage) return name
  }
  return undefined
}

export function toModelRef(ref: ProtoModelRef | undefined): ModelRef {
  return { providerId: ref?.providerId ?? '', modelId: ref?.modelId ?? '' }
}

export function toCatalogModel(info: ProtoModelInfo): CatalogModel {
  return {
    ref: toModelRef(info.ref),
    label: info.label,
    vision: info.vision,
    structuredOutput: info.structuredOutput,
    // A stage this build does not know is skipped rather than invented, like everywhere
    // else the enum crosses.
    stages: info.stages.flatMap((stage) => {
      const name = stageFromProto(stage)
      return name ? [name] : []
    }),
    disabled: info.disabled,
    disabledReason: info.disabledReason,
    contextTokens: info.contextTokens,
    inputUsdPerMillion: info.inputUsdPerMillion,
    outputUsdPerMillion: info.outputUsdPerMillion,
    pricingCheckedAt: info.pricingCheckedAt,
    requiredCredits: info.requiredCredits,
    affordable: info.affordable,
  }
}

export function toAdminCatalogEntry(entry: ProtoCatalogEntry): AdminCatalogEntry {
  return {
    modelId: entry.modelId,
    providerSlug: entry.providerSlug,
    label: entry.label,
    description: entry.description,
    vision: entry.vision,
    structuredOutput: entry.structuredOutput,
    contextTokens: entry.contextTokens,
    inputUsdPerMillion: entry.inputUsdPerMillion,
    outputUsdPerMillion: entry.outputUsdPerMillion,
    curated: entry.curated,
    // Same posture as the reasoning fallback below: a purpose slug a newer server sends
    // that this build has no tab for is dropped rather than rendered blank.
    purposes: entry.purposes.filter(isModelPurpose),
    imageOutput: entry.imageOutput,
    videoOutput: entry.videoOutput,
    listed: entry.listed,
    // A server newer than this build could name an effort this one has no control for.
    // Falling back to "no override" is the honest render: it is what the stage policy does.
    reasoningEffort: isReasoningEffort(entry.reasoningEffort) ? entry.reasoningEffort : '',
    reasoning: {
      reasons: entry.reasons,
      // Same defensive posture as the fallback above: a value a newer server publishes that
      // this build has no control for is dropped rather than offered as an unrenderable
      // option. Order is preserved — it is the source's descending effort order.
      efforts: entry.reasoningEfforts.filter(isReasoningEffort),
      defaultEffort: isReasoningEffort(entry.reasoningDefaultEffort)
        ? entry.reasoningDefaultEffort
        : '',
      mandatory: entry.reasoningMandatory,
      nativeEffort: entry.reasoningNativeEffort,
      maxTokens: entry.reasoningMaxTokens,
      drifted: entry.reasoningDrifted,
      known: entry.reasoningKnown,
    },
    reasoningSpend: entry.reasoningSpend
      ? {
          calls: entry.reasoningSpend.calls,
          reasoningTokens: entry.reasoningSpend.reasoningTokens,
          completionTokens: entry.reasoningSpend.completionTokens,
          reasoningTruncations: entry.reasoningSpend.reasoningTruncations,
        }
      : undefined,
    sourceCreatedAt: entry.sourceCreatedAt,
  }
}

export function toCatalogBrowse(response: ProtoListCatalogResponse): CatalogBrowse {
  return {
    entries: response.entries.map(toAdminCatalogEntry),
    fetchedAt: response.fetchedAt,
    fromCache: response.fromCache,
    fetchError: response.fetchError,
  }
}

/** Undefined for a stage this build does not know — skipped rather than shown blank. */
export function toStageSelection(selection: ProtoSelection): StageSelection | undefined {
  const stage = stageFromProto(selection.stage)
  if (!stage) return undefined
  return {
    stage,
    ref: toModelRef(selection.ref),
    missing: selection.missing,
    slot: slotFromProto(selection.slot),
  }
}

function slotFromProto(slot: SelectionSlot): SelectionSlotName {
  if (slot === SelectionSlot.CANDIDATE_A) return 'candidateA'
  if (slot === SelectionSlot.CANDIDATE_B) return 'candidateB'
  return 'active'
}

export function toComparisonPair(value: ProtoComparisonPair): ComparisonPair | undefined {
  const stage = stageFromProto(value.stage)
  if (!stage) return undefined
  return {
    stage,
    candidateA: value.candidateA ? toStageSelection(value.candidateA) : undefined,
    candidateB: value.candidateB ? toStageSelection(value.candidateB) : undefined,
  }
}

export function toRecommendationSet(value: ProtoRecommendationSet): RecommendationSet {
  return {
    id: value.id,
    label: value.label,
    selections: value.selections.flatMap((selection) => {
      const stage = stageFromProto(selection.stage)
      return stage
        ? [
            {
              stage,
              active: toModelRef(selection.active),
              candidateA: toModelRef(selection.candidateA),
              candidateB: toModelRef(selection.candidateB),
            },
          ]
        : []
    }),
  }
}

export function listModelsQueryKey(transport: Transport) {
  return createConnectQueryKey({
    schema: ProviderService.method.listModels,
    input: {},
    transport,
    cardinality: 'finite',
  })
}

export function getSelectionsQueryKey(transport: Transport) {
  return createConnectQueryKey({
    schema: ProviderService.method.getSelections,
    input: {},
    transport,
    cardinality: 'finite',
  })
}

export function getComparisonPairsQueryKey(transport: Transport) {
  return createConnectQueryKey({
    schema: ProviderService.method.getComparisonPairs,
    input: {},
    transport,
    cardinality: 'finite',
  })
}

export function listRecommendationSetsQueryKey(transport: Transport) {
  return createConnectQueryKey({
    schema: ProviderService.method.listRecommendationSets,
    input: {},
    transport,
    cardinality: 'finite',
  })
}
