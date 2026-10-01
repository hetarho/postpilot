import type { ExperimentCandidate } from './types'

export interface CandidateSide {
  candidate: ExperimentCandidate
  /** The blind name the whole screen refers to — A through E. */
  label: string
}

/** Orders the candidates by the server's blind side assignment, so A through E mean the same candidate
 *  on every render and every reload — a comparison whose sides can swap is worthless.
 *
 *  It lives with the entity rather than with the comparison widget because three layers need
 *  the same answer: the widget labels its panels, the page docks the candidate switch in the
 *  thumb band (THEME-24), and the review names the candidates without
 *  revealing which model either one is. */
export function candidateSides(candidates: readonly ExperimentCandidate[]): CandidateSide[] {
  const order = { left: 0, right: 1, c: 2, d: 3, e: 4 }
  return [...candidates]
    .sort((a, b) => order[a.displaySide] - order[b.displaySide])
    .map((candidate, index) => ({ candidate, label: ['A', 'B', 'C', 'D', 'E'][index] ?? '?' }))
}
