import type { ModelRef } from './types'

/** An optional, page-owned verdict on whether each listed model may be used for THIS workflow
 *  (T112). It is deliberately not a capability flag: loading and a failed read are surface
 *  states of the whole list, told apart from a model's own stable reason. A model the ready
 *  verdict does not resolve is unusable (never eligible by default). */
export type ModelAvailability =
  | { kind: 'loading' }
  | { kind: 'failed'; retry?: () => void }
  | { kind: 'ready'; resolve: (ref: ModelRef) => ModelVerdict }

export type ModelVerdict = { usable: true } | { usable: false; reason: string }

export const UNRESOLVED: ModelVerdict = { usable: false, reason: '' }

/** The verdict for one model, closed while the list is not ready. */
export function verdictOf(
  availability: ModelAvailability | undefined,
  ref: ModelRef,
): ModelVerdict {
  if (!availability) return { usable: true }
  if (availability.kind !== 'ready') return UNRESOLVED
  return availability.resolve(ref)
}
