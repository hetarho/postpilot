import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import { ConfigurationAuthoringService as Service, ProtoConfigurationKind } from '@/shared/api'
import { AUTHORING_KINDS, type AuthoringScope } from '../model/types'
import {
  authoringKindFromProto,
  authoringKindToProto,
  mapAuthoringEstimate,
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
