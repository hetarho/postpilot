import type { ExperimentCandidate } from './types'

export interface CandidateSide {
  candidate: ExperimentCandidate
  /** The blind name the whole screen refers to — 'A' or 'B'. */
  label: string
}

/** Orders the candidates by the server's blind side assignment, so A and B mean the same candidate
 *  on every render and every reload — a comparison whose sides can swap is worthless.
 *
 *  It lives with the entity rather than with the comparison widget because three layers need
 *  the same answer: the widget labels its panels A and B, the page docks the A/B switch in the
 *  thumb band (design-language §4.3), and the verdict sheet names the two candidates without
 *  revealing which model either one is. */
export function candidateSides(candidates: readonly ExperimentCandidate[]): CandidateSide[] {
  return [...candidates]
    .sort((a, b) => a.displaySide.localeCompare(b.displaySide))
    .map((candidate, index) => ({ candidate, label: index === 0 ? 'A' : 'B' }))
}
