import type { ExperimentCandidate } from '@/entities/model-experiment'

/** Return a complete dense ranking, including ties, in the stable blind display order. */
export function completeRanks(
  candidates: readonly ExperimentCandidate[],
  chosen: Readonly<Record<string, number>>,
): Array<{ candidateId: string; rank: number }> | undefined {
  const succeeded = candidates.filter((candidate) => candidate.status === 'succeeded')
  if (succeeded.length < 2) return undefined
  const ranks = succeeded.map((candidate) => chosen[candidate.id] ?? 0)
  if (ranks.some((rank) => !Number.isInteger(rank) || rank < 1 || rank > succeeded.length))
    return undefined
  const distinct = [...new Set(ranks)].sort((a, b) => a - b)
  if (distinct.some((rank, index) => rank !== index + 1)) return undefined
  return succeeded.map((candidate, index) => ({ candidateId: candidate.id, rank: ranks[index] }))
}
