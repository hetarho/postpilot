import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import { ProtoReplacementSurface, ReplacementCandidateSchema } from '@/shared/api'
import {
  replacementSurfaceFromProto,
  replacementSurfaceToProto,
  toReplacementCandidate,
} from './replacement-mappers'

// ARCH-3: the mirror is walked against the generated enum, so a surface the contract gains
// without a name here fails this file rather than a mark.
describe('the replacement surface mirror', () => {
  it('maps every generated surface both ways', () => {
    const wire = Object.values(ProtoReplacementSurface).filter(
      (value): value is ProtoReplacementSurface =>
        typeof value === 'number' && value !== ProtoReplacementSurface.UNSPECIFIED,
    )
    expect(wire.map(replacementSurfaceFromProto)).toEqual(['title', 'tag', 'body'])
    for (const value of wire)
      expect(replacementSurfaceToProto(replacementSurfaceFromProto(value)!)).toBe(value)
  })

  it.each([
    ['UNSPECIFIED', ProtoReplacementSurface.UNSPECIFIED],
    ['an unknown number', 9_999 as ProtoReplacementSurface],
  ])('reads %s as no surface, and drops a candidate carrying it', (_label, surface) => {
    expect(replacementSurfaceFromProto(surface)).toBeUndefined()
    expect(
      toReplacementCandidate(create(ReplacementCandidateSchema, { surface, source: '제주' })),
    ).toBeUndefined()
  })
})
