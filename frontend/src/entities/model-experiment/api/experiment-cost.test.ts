import { describe, expect, it } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { CostSource, LeaderboardEntrySchema } from '@/shared/api'
import { toLeaderboardEntry } from './experiment-mappers'

describe('cost quality crossing the wire', () => {
  // QUOTA-66: the server sends no cost to anyone but the operator, which arrives as UNSPECIFIED
  // and must read as withheld, never as a missing figure the screen would call unavailable.
  it('reads an unsent cost as withheld', () => {
    const mapped = toLeaderboardEntry(create(LeaderboardEntrySchema, { modelLabel: 'A' }))
    expect(mapped.costQuality).toBe('withheld')
  })

  // ARCH-3: every value the wire names has its own client name; none falls through.
  it('names every wire value distinctly', () => {
    const names = Object.values(CostSource)
      .filter((value): value is CostSource => typeof value === 'number')
      .map(
        (value) =>
          toLeaderboardEntry(create(LeaderboardEntrySchema, { costQuality: value })).costQuality,
      )
    expect(new Set(names).size).toBe(names.length)
  })
})
