import { create } from '@bufbuild/protobuf'
import { expect, it } from 'vitest'
import { LeaderboardEntrySchema, ModelRefSchema } from '@/shared/api'
import { toLeaderboardEntry } from './experiment-mappers'

it('maps one ranked evaluation separately from its pairwise wins, losses and draws', () => {
  const entry = toLeaderboardEntry(
    create(LeaderboardEntrySchema, {
      model: create(ModelRefSchema, { providerId: 'p', modelId: 'a' }),
      rating: 1516,
      evaluatedComparisons: 1,
      matches: 4,
      wins: 2,
      losses: 1,
      draws: 1,
      provisional: true,
    }),
  )
  expect(entry).toMatchObject({
    rating: 1516,
    evaluatedComparisons: 1,
    matches: 4,
    wins: 2,
    losses: 1,
    draws: 1,
    provisional: true,
  })
})
