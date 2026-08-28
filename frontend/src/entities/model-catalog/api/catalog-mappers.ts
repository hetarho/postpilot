// Proto ↔ domain for the model catalog. The stage enum crosses here and nowhere else.
import type { Transport } from '@connectrpc/connect'
import { createConnectQueryKey } from '@connectrpc/connect-query'
import {
  type ProtoModelInfo,
  type ProtoModelRef,
  type ProtoSelection,
  ProviderService,
  Stage,
} from '@/shared/api'
import type { CatalogModel, ModelRef, StageName, StageSelection } from '../model/types'

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
  }
}

/** Undefined for a stage this build does not know — skipped rather than shown blank. */
export function toStageSelection(selection: ProtoSelection): StageSelection | undefined {
  const stage = stageFromProto(selection.stage)
  if (!stage) return undefined
  return { stage, ref: toModelRef(selection.ref), missing: selection.missing }
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
