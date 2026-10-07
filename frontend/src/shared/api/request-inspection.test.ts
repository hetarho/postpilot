import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import {
  FragmentAuthorship,
  InspectionRole,
  InspectionStatus,
  RequestInspectionSchema,
} from './gen/postpilot/v1/request_inspection_pb'
import {
  fragmentAuthorshipNames,
  inspectionRoleNames,
  inspectionStatusNames,
  requestInspectionFromProto,
} from './request-inspection'

const captured = () =>
  create(RequestInspectionSchema, {
    version: 1,
    status: InspectionStatus.CAPTURED,
    stage: 'write',
    mode: 'direct',
    promptVersion: 'write-v1',
    schemaVersion: 'post-v1',
    output: { name: 'post', version: 'post-v1', schema: '{}' },
    fragments: [
      {
        id: 'contract',
        role: InspectionRole.USER,
        authorship: FragmentAuthorship.CODE,
        materialRole: 'contract',
        text: '첫째 🍊',
        sourceRefs: [],
      },
      {
        id: 'memo',
        role: InspectionRole.SYSTEM,
        authorship: FragmentAuthorship.ACCOUNT,
        materialRole: 'fact',
        text: 'owner\ntext',
        sourceRefs: ['memo:1'],
      },
    ],
    conditions: { maxCompletionTokens: 8192n, structuredOutput: false, reasoningEffort: 'none' },
    measures: { characters: 7n, utf8Bytes: 15n, providerPromptTokens: 0n },
    issuedAt: { seconds: 1n, nanos: 0 },
  })

describe('safe request inspection projection', () => {
  it.each([
    [InspectionStatus, inspectionStatusNames],
    [InspectionRole, inspectionRoleNames],
    [FragmentAuthorship, fragmentAuthorshipNames],
  ])('pins each generated enum to an explicit mapping', (generated, names) => {
    expect(Object.keys(names).map(Number)).toEqual(
      Object.values(generated).filter((v) => typeof v === 'number'),
    )
  })

  it('preserves exact ordered material, author/role independence and optional reported zero', () => {
    const result = requestInspectionFromProto(captured())
    expect(result.status).toBe('captured')
    expect(result.fragments.map((f) => [f.id, f.role, f.authorship, f.text])).toEqual([
      ['contract', 'user', 'code', '첫째 🍊'],
      ['memo', 'system', 'account', 'owner\ntext'],
    ])
    expect(result.conditions?.structuredOutput).toBe(false)
    expect(result.measures?.providerPromptTokens).toBe(0n)
    expect(result.measures?.providerCompletionTokens).toBeUndefined()
    expect(result.measures?.referenceTokenEstimate).toBeUndefined()
  })

  it('reads absent/unsupported history as unavailable without inferred payload', () => {
    expect(requestInspectionFromProto().status).toBe('unavailable')
    const value = captured()
    value.status = 999 as InspectionStatus
    expect(requestInspectionFromProto(value)).toEqual({
      version: 1,
      status: 'unavailable',
      stage: 'write',
      mode: 'direct',
      fragments: [],
      selectedRuleIds: [],
    })
    value.status = InspectionStatus.UNAVAILABLE
    expect(requestInspectionFromProto(value).fragments).toEqual([])
  })

  it('never presents runtime usage or execution time as a preview/current read', () => {
    const value = captured()
    value.status = InspectionStatus.PREPARED
    expect(requestInspectionFromProto(value).status).toBe('unavailable')
    value.issuedAt = undefined
    value.measures = undefined
    expect(requestInspectionFromProto(value).status).toBe('prepared')
    value.status = InspectionStatus.CURRENT
    expect(requestInspectionFromProto(value).status).toBe('current')
  })

  it('copies only product fields even when an unsafe transport object carries extra properties', () => {
    const value = Object.assign(captured(), {
      apiKey: 'secret',
      baseUrl: 'endpoint',
      costMicrousd: 9n,
      media: new Uint8Array([1]),
    })
    const result = requestInspectionFromProto(value)
    expect(Object.keys(result)).not.toContain('apiKey')
    expect(Object.keys(result)).not.toContain('baseUrl')
    expect(Object.keys(result)).not.toContain('costMicrousd')
    expect(Object.keys(result)).not.toContain('media')
  })

  it.each([
    (value: ReturnType<typeof captured>) => {
      value.output!.name = ' '
    },
    (value: ReturnType<typeof captured>) => {
      value.promptVersion = ' '
    },
    (value: ReturnType<typeof captured>) => {
      value.fragments[1]!.id = value.fragments[0]!.id
    },
    (value: ReturnType<typeof captured>) => {
      value.fragments[0]!.materialRole = ''
    },
    (value: ReturnType<typeof captured>) => {
      value.fragments[0]!.sourceRefs = ['memo', 'memo']
    },
    (value: ReturnType<typeof captured>) => {
      value.selectedRuleIds = [' ']
    },
    (value: ReturnType<typeof captured>) => {
      value.conditions!.model = createModelWithMissingIdentity()
    },
    (value: ReturnType<typeof captured>) => {
      value.conditions!.maxCompletionTokens = 0n
    },
    (value: ReturnType<typeof captured>) => {
      value.conditions!.reasoningEffort = 'future-effort'
    },
    (value: ReturnType<typeof captured>) => {
      value.measures!.providerPromptTokens = -1n
    },
    (value: ReturnType<typeof captured>) => {
      value.issuedAt!.seconds = 253402300800n
    },
    (value: ReturnType<typeof captured>) => {
      value.issuedAt!.nanos = 1_000_000_000
    },
    (value: ReturnType<typeof captured>) => {
      value.fragments[0]!.text = '\ud800'
    },
    (value: ReturnType<typeof captured>) => {
      value.output!.schema = '\udfff'
    },
  ])('keeps malformed effective conditions and request identities unavailable', (mutate) => {
    const value = captured()
    mutate(value)
    expect(requestInspectionFromProto(value).status).toBe('unavailable')
  })
})

function createModelWithMissingIdentity() {
  return { $typeName: 'postpilot.v1.ModelRef' as const, providerId: '', modelId: 'model' }
}
