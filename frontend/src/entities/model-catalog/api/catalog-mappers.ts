// Proto ↔ domain for the model catalog. The stage enum crosses here and nowhere else.
import type { Transport } from '@connectrpc/connect'
import { createConnectQueryKey } from '@connectrpc/connect-query'
import {
  type ProtoModelInfo,
  type ProtoModelRef,
  type ProtoSelection,
  ProviderService,
  SelectionSlot,
  Stage,
  type ProtoComparisonPair,
  type ProtoRecommendationSet,
} from '@/shared/api'
import { planFromProto } from '@/entities/plan/@x/model-catalog'
import type {
  CatalogModel,
  ComparisonPair,
  ModelRef,
  RecommendationSet,
  SelectionSlotName,
  StageName,
  StageSelection,
} from '../model/types'

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
    disabled: info.disabled,
    disabledReason: info.disabledReason,
    contextTokens: info.contextTokens,
    inputUsdPerMillion: info.inputUsdPerMillion,
    outputUsdPerMillion: info.outputUsdPerMillion,
    pricingCheckedAt: info.pricingCheckedAt,
    minPlan: planFromProto(info.minPlan),
    locked: info.locked,
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
