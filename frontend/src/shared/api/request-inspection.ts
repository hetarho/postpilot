import {
  FragmentAuthorship,
  InspectionRole,
  InspectionStatus,
  type RequestInspection,
} from './gen/postpilot/v1/request_inspection_pb'

export const inspectionStatusNames = {
  [InspectionStatus.UNSPECIFIED]: undefined,
  [InspectionStatus.CURRENT]: 'current',
  [InspectionStatus.PREPARED]: 'prepared',
  [InspectionStatus.CAPTURED]: 'captured',
  [InspectionStatus.UNAVAILABLE]: 'unavailable',
} as const
export const inspectionRoleNames = {
  [InspectionRole.UNSPECIFIED]: undefined,
  [InspectionRole.SYSTEM]: 'system',
  [InspectionRole.USER]: 'user',
  [InspectionRole.ASSISTANT]: 'assistant',
} as const
export const fragmentAuthorshipNames = {
  [FragmentAuthorship.UNSPECIFIED]: undefined,
  [FragmentAuthorship.CODE]: 'code',
  [FragmentAuthorship.ACCOUNT]: 'account',
} as const

export type RequestInspectionStatus = Exclude<
  (typeof inspectionStatusNames)[keyof typeof inspectionStatusNames],
  undefined
>
export interface RequestInspectionView {
  version: number
  status: RequestInspectionStatus
  stage: string
  mode: string
  promptVersion?: string
  schemaVersion?: string
  fragments: {
    id: string
    role: 'system' | 'user' | 'assistant'
    authorship: 'code' | 'account'
    materialRole: string
    text: string
    sourceRefs: string[]
  }[]
  selectedRuleIds: string[]
  output?: { name: string; version: string; schema: string }
  conditions?: {
    model?: { providerId: string; modelId: string }
    maxCompletionTokens?: bigint
    reasoningEffort?: string
    structuredOutput?: boolean
  }
  measures?: {
    characters?: bigint
    utf8Bytes?: bigint
    referenceTokenEstimate?: bigint
    providerPromptTokens?: bigint
    providerCompletionTokens?: bigint
    providerReasoningTokens?: bigint
  }
  issuedAt?: { seconds: bigint; nanos: number }
}

function scalarText(value: string): boolean {
  for (const character of value) {
    const unit = character.charCodeAt(0)
    if (character.length === 1 && unit >= 0xd800 && unit <= 0xdfff) return false
  }
  return true
}

function inspectionName(value: string): boolean {
  return scalarText(value) && value.trim() !== ''
}

function inspectionNames(values: readonly string[]): boolean {
  return values.every(inspectionName) && new Set(values).size === values.length
}

/** Decode only published product fields. Historical absence is never reconstructed. */
export function requestInspectionFromProto(value?: RequestInspection): RequestInspectionView {
  const unavailable: RequestInspectionView = {
    version: 1,
    status: 'unavailable',
    stage: value && scalarText(value.stage) ? value.stage : '',
    mode: value && scalarText(value.mode) ? value.mode : '',
    fragments: [],
    selectedRuleIds: [],
  }
  if (!value) return unavailable
  const status = inspectionStatusNames[value.status]
  if (!status || status === 'unavailable' || value.version !== 1) return unavailable
  if (
    !value.output ||
    ![
      value.stage,
      value.mode,
      value.promptVersion,
      value.schemaVersion,
      value.output.name,
      value.output.version,
    ].every(inspectionName)
  )
    return unavailable
  if (!scalarText(value.output.schema)) return unavailable
  if (!inspectionNames(value.selectedRuleIds)) return unavailable
  if (value.conditions) {
    const { model, maxCompletionTokens, reasoningEffort } = value.conditions
    if (model && ![model.providerId, model.modelId].every(inspectionName)) return unavailable
    if (maxCompletionTokens !== undefined && maxCompletionTokens <= 0n) return unavailable
    if (
      reasoningEffort !== undefined &&
      !['', 'unset', 'none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max'].includes(
        reasoningEffort,
      )
    )
      return unavailable
  }
  if (
    value.measures &&
    [
      value.measures.characters,
      value.measures.utf8Bytes,
      value.measures.referenceTokenEstimate,
      value.measures.providerPromptTokens,
      value.measures.providerCompletionTokens,
      value.measures.providerReasoningTokens,
    ].some((count) => count !== undefined && count < 0n)
  )
    return unavailable
  if (value.issuedAt) {
    const { seconds, nanos } = value.issuedAt
    if (
      seconds < -62135596800n ||
      seconds > 253402300799n ||
      !Number.isInteger(nanos) ||
      nanos < 0 ||
      nanos >= 1_000_000_000 ||
      (seconds === -62135596800n && nanos === 0)
    )
      return unavailable
  }
  if (
    status !== 'captured' &&
    (value.issuedAt ||
      value.measures?.providerPromptTokens !== undefined ||
      value.measures?.providerCompletionTokens !== undefined ||
      value.measures?.providerReasoningTokens !== undefined)
  )
    return unavailable
  const fragments: RequestInspectionView['fragments'] = []
  const fragmentIds = new Set<string>()
  for (const fragment of value.fragments) {
    const role = inspectionRoleNames[fragment.role]
    const authorship = fragmentAuthorshipNames[fragment.authorship]
    if (
      !role ||
      !authorship ||
      !inspectionName(fragment.id) ||
      fragmentIds.has(fragment.id) ||
      !inspectionName(fragment.materialRole) ||
      !inspectionNames(fragment.sourceRefs) ||
      !scalarText(fragment.text)
    )
      return unavailable
    fragmentIds.add(fragment.id)
    fragments.push({
      id: fragment.id,
      role,
      authorship,
      materialRole: fragment.materialRole,
      text: fragment.text,
      sourceRefs: [...fragment.sourceRefs],
    })
  }
  return {
    version: value.version,
    status,
    stage: value.stage,
    mode: value.mode,
    promptVersion: value.promptVersion,
    schemaVersion: value.schemaVersion,
    fragments,
    selectedRuleIds: [...value.selectedRuleIds],
    output: { name: value.output.name, version: value.output.version, schema: value.output.schema },
    conditions: value.conditions && {
      model: value.conditions.model && {
        providerId: value.conditions.model.providerId,
        modelId: value.conditions.model.modelId,
      },
      maxCompletionTokens: value.conditions.maxCompletionTokens,
      reasoningEffort: value.conditions.reasoningEffort,
      structuredOutput: value.conditions.structuredOutput,
    },
    measures: value.measures && {
      characters: value.measures.characters,
      utf8Bytes: value.measures.utf8Bytes,
      referenceTokenEstimate: value.measures.referenceTokenEstimate,
      providerPromptTokens: value.measures.providerPromptTokens,
      providerCompletionTokens: value.measures.providerCompletionTokens,
      providerReasoningTokens: value.measures.providerReasoningTokens,
    },
    issuedAt: value.issuedAt && { seconds: value.issuedAt.seconds, nanos: value.issuedAt.nanos },
  }
}
