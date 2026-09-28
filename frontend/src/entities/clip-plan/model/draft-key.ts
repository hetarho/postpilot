import type { ClipEditPlan } from './edit-plan'

/** A plan's meaning as one string: what the server resolves and what the evidence records are
 *  left out, and object keys are sorted, so two reads of the same plan compare equal. */
export function clipDraftKey(plan: ClipEditPlan) {
  return JSON.stringify(plan, (key, value: unknown) => {
    if (
      [
        'resolvedStartMs',
        'resolvedEndMs',
        'effectiveStartMs',
        'effectiveEndMs',
        'evidence',
        'fallbackReason',
      ].includes(key)
    )
      return undefined
    // Object insertion order can change across a transport round trip or clone.
    // Only semantic changes should enter history or schedule another save.
    if (value && typeof value === 'object' && !Array.isArray(value))
      return Object.fromEntries(Object.entries(value).sort(([a], [b]) => a.localeCompare(b)))
    return typeof value === 'number' && !Number.isFinite(value) ? String(value) : value
  })
}
