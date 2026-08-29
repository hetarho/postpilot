import type { ExperimentCandidate } from '@/entities/model-experiment'

export interface CandidateSide {
  candidate: ExperimentCandidate
  /** The blind name the whole screen refers to — 'A' or 'B'. */
  label: string
}

/** Orders the candidates by the server's blind side assignment, so A and B mean the same candidate
 *  on every render and every reload — a comparison whose sides can swap is worthless.
 *
 *  It is part of the slice's public API rather than a local sort because the page docks the A/B
 *  switch in the thumb band (design-language §4.3) and has to label it the same way the panels do.
 */
export function candidateSides(candidates: readonly ExperimentCandidate[]): CandidateSide[] {
  return [...candidates]
    .sort((a, b) => a.displaySide.localeCompare(b.displaySide))
    .map((candidate, index) => ({ candidate, label: index === 0 ? 'A' : 'B' }))
}
