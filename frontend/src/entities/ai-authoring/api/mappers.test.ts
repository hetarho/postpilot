import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import {
  ConfigurationAuthoringService as Service,
  ProtoConfigurationKind,
  ProtoAuthoringDraftState,
} from '@/shared/api'
import { AUTHORING_KINDS, type AuthoringScope } from '../model/types'
import {
  authoringKindFromProto,
  authoringKindToProto,
  mapAuthoringEstimate,
  authoringDraftStateFromProto,
  mapAuthoringSession,
  type WireSession,
} from './mappers'
const scope: AuthoringScope = { ownerId: 'alice', kind: 'post-template', targetId: 'owned' }
function wire(patch: Partial<WireSession> = {}): WireSession {
  const result = create(Service.method.getAuthoringSession.output, {
    session: {
      id: 'session',
      kind: ProtoConfigurationKind.POST_TEMPLATE,
      revision: 1,
      phase: 'choosing',
      targetId: 'owned',
    },
  }).session!
  return Object.assign(result, patch)
}
describe('strict authoring boundary', () => {
  it.each(AUTHORING_KINDS)('maps %s without a fallback kind', (kind) =>
    expect(authoringKindFromProto(authoringKindToProto(kind))).toBe(kind),
  )
  it('rejects unknown kinds and identity/version/phase mismatches', () => {
    expect(() => authoringKindFromProto(ProtoConfigurationKind.UNSPECIFIED)).toThrow()
    for (const bad of [
      wire({ id: '' }),
      wire({ targetId: 'foreign' }),
      wire({ kind: ProtoConfigurationKind.WRITING_VOICE }),
      wire({ revision: -1 }),
      wire({ phase: 'unknown' }),
    ])
      expect(() => mapAuthoringSession(bad, scope)).toThrow()
  })
  it('publishes eight complete unique peers as a group and rejects partial output', () => {
    const candidates = Array.from({ length: 8 }, (_, i) => ({
      id: `candidate-${i}`,
      name: '읽을 이름',
      description: '짧은 설명',
      body: '<write>글</write>',
      titleArea: '',
    }))
    expect(
      mapAuthoringSession(wire({ candidates: candidates as WireSession['candidates'] }), scope)
        .candidates,
    ).toHaveLength(8)
    expect(() =>
      mapAuthoringSession(
        wire({ candidates: candidates.slice(0, 7) as WireSession['candidates'] }),
        scope,
      ),
    ).toThrow()
    expect(() =>
      mapAuthoringSession(
        wire({
          candidates: candidates.map((item) => ({
            ...item,
            id: 'duplicate',
          })) as WireSession['candidates'],
        }),
        scope,
      ),
    ).toThrow()
  })
  it('never accepts a saved response without a matching confirmed record', () => {
    expect(() => mapAuthoringSession(wire({ phase: 'saved' }), scope)).toThrow()
    const record = create(Service.method.getAuthoringSession.output, {
      session: {
        saved: { kind: ProtoConfigurationKind.VIDEO_GUIDELINE, id: 'wrong', name: 'wrong' },
      },
    }).session!.saved!
    const saved = wire({ phase: 'saved', saved: record })
    expect(() => mapAuthoringSession(saved, scope)).toThrow()
  })
  it('keeps zero, free, unknown and unsafe credit figures distinct', () => {
    expect(mapAuthoringEstimate({ free: true })).toEqual({ free: true, credits: 0 })
    expect(mapAuthoringEstimate({ free: false, credits: 0n })).toEqual({ free: false, credits: 0 })
    for (const credits of [undefined, -1n, BigInt(Number.MAX_SAFE_INTEGER) + 1n])
      expect(() => mapAuthoringEstimate({ free: false, credits })).toThrow()
  })
})

it.each([2, 4, 8, 16])(
  'requires every explicitly requested %d candidate and retains bounded invalid manual source',
  (count) => {
    const candidates = Array.from({ length: count }, (_, i) => ({
      id: `c-${i}`,
      revision: 3,
      name: `n${i}`,
      description: '',
      body: `body${i}`,
      titleArea: '',
    }))
    const result = wire({
      candidateCount: count,
      candidates: candidates as WireSession['candidates'],
    })
    expect(mapAuthoringSession(result, scope).candidates).toHaveLength(count)
    result.candidates.pop()
    expect(() => mapAuthoringSession(result, scope)).toThrow()
    const retained = wire({
      phase: 'editing',
      workingSource: {
        id: 'draft',
        name: 'unfinished',
        body: '',
        titleArea: '',
        description: '',
        revision: 4,
      } as WireSession['workingSource'],
      draftState: ProtoAuthoringDraftState.INCOMPLETE,
      selected: {
        id: 'draft',
        name: 'last-valid',
        body: 'old body',
        titleArea: '',
        description: '',
        revision: 2,
      } as WireSession['selected'],
    })
    const mapped = mapAuthoringSession(retained, scope)
    expect(mapped.draftState).toBe('incomplete')
    expect(mapped.workingSource?.body).toBe('')
    expect(mapped.selected?.body).toBe('old body')
  },
)
it('pins every wire draft state and rejects an unknown value', () => {
  for (const value of Object.values(ProtoAuthoringDraftState).filter((v) => typeof v === 'number'))
    expect(() => authoringDraftStateFromProto(value)).not.toThrow()
  expect(() => authoringDraftStateFromProto(999 as ProtoAuthoringDraftState)).toThrow()
})

it('retains a valid untitled saved guideline instead of turning the read into a missing draft', () => {
  const selected = {
    id: 'owned',
    name: '',
    description: '',
    body: 'Readable saved direction',
    titleArea: '',
    revision: 1,
  }
  const incoming = wire({
    kind: ProtoConfigurationKind.POST_GUIDELINE,
    phase: 'editing',
    savedAvailable: true,
    selected: selected as WireSession['selected'],
  })
  expect(mapAuthoringSession(incoming, { ...scope, kind: 'post-guideline' }).selected?.name).toBe(
    '',
  )
})

it('keeps a captured personal voice readable while it has no synthetic example yet', () => {
  const selected = {
    id: 'personal',
    name: '내 말투',
    description: '저장한 문장 특징',
    body: '',
    titleArea: '',
    revision: 0,
  }
  const incoming = wire({
    kind: ProtoConfigurationKind.WRITING_VOICE,
    phase: 'editing',
    targetId: 'personal',
    savedAvailable: true,
    draftState: ProtoAuthoringDraftState.INCOMPLETE,
    selected: selected as WireSession['selected'],
    savedBaseline: selected as WireSession['savedBaseline'],
  })
  const mapped = mapAuthoringSession(incoming, {
    ownerId: 'alice',
    kind: 'writing-voice',
    targetId: 'personal',
  })
  expect(mapped.selected?.body).toBe('')
  expect(mapped.draftState).toBe('incomplete')
  expect(mapped.savedAvailable).toBe(true)
})

it('does not turn a known deleted-target conflict back into saved availability', () => {
  const selected = {
    id: 'owned',
    name: 'saved',
    description: '',
    body: 'saved body',
    titleArea: '',
    revision: 1,
  }
  const incoming = wire({
    phase: 'saving',
    savedAvailable: false,
    failureReason: 'AUTHORING_SAVE_CONFLICT',
    selected: selected as WireSession['selected'],
    savedBaseline: selected as WireSession['savedBaseline'],
  })
  expect(mapAuthoringSession(incoming, scope).savedAvailable).toBe(false)
})

it('recovers a new published setting under its saved id without changing receipt scope or voice semantics', () => {
  const incoming = wire({
    targetId: '',
    phase: 'editing',
    savedAvailable: true,
    hasUnpublishedChanges: true,
    saved: create(Service.method.getAuthoringSession.output, {
      session: { saved: { id: 'published', kind: ProtoConfigurationKind.POST_TEMPLATE } },
    }).session!.saved,
  })
  expect(mapAuthoringSession(incoming, { ...scope, targetId: 'published' }).targetId).toBe(
    'published',
  )
  expect(incoming.targetId).toBe('')
  expect(mapAuthoringSession(incoming, { ...scope, targetId: '' }).targetId).toBe('')
  expect(() => mapAuthoringSession(incoming, { ...scope, targetId: 'another' })).toThrow()
  incoming.kind = ProtoConfigurationKind.WRITING_VOICE
  incoming.saved!.kind = ProtoConfigurationKind.WRITING_VOICE
  expect(() =>
    mapAuthoringSession(incoming, { ...scope, kind: 'writing-voice', targetId: 'published' }),
  ).toThrow()
})
