import { refKey, type ComparisonPair, type ModelRef } from '@/entities/model-catalog'

/** Only refs that the server has confirmed and the screen still shows may start. */
export function labCandidateRefs(
  pair: ComparisonPair | undefined,
  shown: readonly string[] | undefined,
): ModelRef[] | undefined {
  if (!pair?.candidateA || !pair.candidateB || !shown) return undefined
  const selections = [pair.candidateA, pair.candidateB, ...pair.extraCandidates]
  const keys = selections.map((item) => refKey(item.ref))
  if (
    shown.length !== pair.extraCandidates.length ||
    shown.some((key, index) => key !== keys[index + 2])
  )
    return undefined
  if (
    selections.some((item) => item.missing || item.unavailableReason) ||
    new Set(keys).size !== keys.length
  )
    return undefined
  return selections.map((item) => item.ref)
}
