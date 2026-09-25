import { describe, expect, it } from 'vitest'
import { BLOG_FIELD_IDS } from '@/entities/blog-field'
import { QUALITY_METRICS } from '@/entities/quality'
import {
  ProtoBlogField,
  ProtoQualityMetric,
  ProtoQualityVerdict,
  ProtoReplacementSurface,
} from '@/shared/api'
import { fromWire, toWire } from './wire-enum'

const VERDICTS = ['over_band', 'within_band', 'below_minimum', 'absent'] as const
const SURFACES = ['title', 'tag', 'body'] as const

// The fakes map by member name, so every mirrored id has to name its member exactly.
describe('the fakes’ wire-enum rule', () => {
  it.each([
    ['BlogField', ProtoBlogField, BLOG_FIELD_IDS],
    ['QualityMetric', ProtoQualityMetric, QUALITY_METRICS],
    ['QualityVerdict', ProtoQualityVerdict, VERDICTS],
    ['ReplacementSurface', ProtoReplacementSurface, SURFACES],
  ] as const)('maps every %s id to a member and back', (_name, wire, ids) => {
    for (const id of ids) {
      const value = toWire(wire, id)
      expect(value).toBeGreaterThan(0)
      expect(fromWire(wire, value, ids)).toBe(id)
    }
    expect(fromWire(wire, 0, ids)).toBeUndefined()
    expect(fromWire(wire, 9_999, ids)).toBeUndefined()
  })

  it('throws for an id that names no member', () => {
    expect(() => toWire(ProtoBlogField, 'space_travel')).toThrow(
      'fake: space_travel names no member',
    )
  })
})
