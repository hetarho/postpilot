import { describe, expect, it } from 'vitest'
import { clipEditingFixture } from '@/test/clip-editing'
import { clipObservationsFixture } from '@/test/clip-observations'
import { observationCutUsage, observationSummary } from './observations'

describe('recorded observation usage', () => {
  it('maps intersections using identity and saved order, excluding touching endpoints', () => {
    const observation = clipObservationsFixture().sources[0]!
    const segment = observation.segments[0]!
    const plan = clipEditingFixture().plan
    const a = plan.cuts[0]!
    plan.cuts = [
      { ...a, id: 'wrong-fingerprint', fingerprint: 'another' },
      { ...a, id: 'wrong-source', sourceId: 'another' },
      { ...a, id: 'touch-start', startMs: 0, endMs: 3500 },
      { ...a, id: 'touch-end', startMs: 10500, endMs: 12000 },
      { ...a, id: 'trimmed', startMs: 4000, endMs: 8000 },
      { ...a, id: 'overlapping', startMs: 9000, endMs: 11000 },
    ]
    expect(observationCutUsage(segment, observation.source, plan)).toEqual([
      { cutId: 'trimmed', number: 5, startMs: 4000, endMs: 8000 },
      { cutId: 'overlapping', number: 6, startMs: 9000, endMs: 10500 },
    ])
    plan.cuts = [plan.cuts[5]!]
    expect(observationCutUsage(segment, observation.source, plan)).toEqual([
      { cutId: 'overlapping', number: 1, startMs: 9000, endMs: 10500 },
    ])
    expect(observationCutUsage(segment, observation.source, undefined)).toEqual([])
  })
  it('uses recorded evidence for summaries and has no invented summary for empty ranges', () => {
    const [a, b] = clipObservationsFixture().sources
    expect(observationSummary(a!)).toBe(a!.segments[0]!.event)
    expect(observationSummary(b!)).toBe('')
    a!.segments[0]!.event = ''
    expect(observationSummary(a!)).toBe('맛있어요')
  })
})
