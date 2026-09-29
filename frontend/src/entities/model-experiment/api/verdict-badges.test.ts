import { describe, expect, it } from 'vitest'
import { VerdictBadge } from '@/shared/api'
import { NEGATIVE_BADGES, POSITIVE_BADGES, badgeAppliesTo } from '../model/badges'
import { badgeFromProto, badgeToProto, badgesToProto } from './experiment-mappers'

describe('the verdict badge catalog', () => {
  // One list on both sides. A badge added to the contract without a name here would drop
  // silently on every verdict that offered it, which is exactly what this catches.
  it('maps every value the wire names, both ways', () => {
    const wireValues = Object.values(VerdictBadge).filter(
      (value): value is VerdictBadge =>
        typeof value === 'number' && value !== VerdictBadge.UNSPECIFIED,
    )
    expect(wireValues).toHaveLength(POSITIVE_BADGES.length + NEGATIVE_BADGES.length + 1)
    for (const wire of wireValues) {
      const name = badgeFromProto(wire)
      expect(name, `wire value ${wire} has no name`).toBeDefined()
      expect(badgeToProto(name!)).toBe(wire)
    }
  })

  it('drops a wire value this build does not know rather than inventing one', () => {
    expect(badgeFromProto(9_999 as VerdictBadge)).toBeUndefined()
  })

  it('builds the payload a verdict carries', () => {
    expect(
      badgesToProto([{ candidateId: 'left', badges: ['fast', 'other'], otherNote: '설명' }]),
    ).toEqual([
      {
        candidateId: 'left',
        badges: [VerdictBadge.FAST, VerdictBadge.OTHER],
        otherNote: '설명',
      },
    ])
  })

  // Only the voice pair is conditional: it belongs to write comparisons alone (MODEL-62), and
  // an observe comparison produces no prose to judge the voice of.
  it('offers the voice pair for write comparisons only', () => {
    expect(badgeAppliesTo('in_voice', 'write')).toBe(true)
    expect(badgeAppliesTo('off_voice', 'write')).toBe(true)
    expect(badgeAppliesTo('in_voice', 'observe')).toBe(false)
    expect(badgeAppliesTo('off_voice', 'observe')).toBe(false)
    // Every other badge is offered on both stages the lab compares.
    for (const badge of [...POSITIVE_BADGES, ...NEGATIVE_BADGES, 'other'] as const) {
      if (badge === 'in_voice' || badge === 'off_voice') continue
      expect(badgeAppliesTo(badge, 'observe'), badge).toBe(true)
      expect(badgeAppliesTo(badge, 'write'), badge).toBe(true)
    }
  })
})
