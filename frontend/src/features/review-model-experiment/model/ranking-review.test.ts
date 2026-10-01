import { expect, it } from 'vitest'
import type { ExperimentCandidate } from '@/entities/model-experiment'
import { completeRanks } from './ranking-review'

const candidates = ['a', 'b', 'c', 'd', 'e'].map((id) => ({
  id,
  status: 'succeeded',
})) as ExperimentCandidate[]

it('requires every success and accepts dense tied groups', () => {
  expect(completeRanks(candidates, { a: 1, b: 1, c: 2, d: 3, e: 3 })).toEqual([
    { candidateId: 'a', rank: 1 },
    { candidateId: 'b', rank: 1 },
    { candidateId: 'c', rank: 2 },
    { candidateId: 'd', rank: 3 },
    { candidateId: 'e', rank: 3 },
  ])
  expect(completeRanks(candidates, { a: 1, b: 1, c: 2, d: 3 })).toBeUndefined()
  expect(completeRanks(candidates, { a: 1, b: 1, c: 3, d: 4, e: 4 })).toBeUndefined()
  expect(
    completeRanks([{ ...candidates[0], status: 'failed' }, candidates[1]], { b: 1 }),
  ).toBeUndefined()
})
