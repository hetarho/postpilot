/** T111's answer for one registered observe model: eligible for clip analysis, or exactly one
 *  of the four stable reasons it is not (CLIP-30, CLIP-44). The server's statuses are the
 *  authority for this page; `videoInput` and `inlineStaticVideo` stay capability badges. */
export const CLIP_ELIGIBILITY_STATUSES = [
  'eligible',
  'video_input_absent',
  'inline_endpoint_unavailable',
  'required_parameters_unsupported',
  'price_ceiling_unavailable',
] as const
export type ClipEligibilityStatus = (typeof CLIP_ELIGIBILITY_STATUSES)[number]
export type ClipIneligibility = Exclude<ClipEligibilityStatus, 'eligible'>

/** Structurally the catalog's ModelRef; kept local so this entity imports no other entity. */
export interface ClipModelRef {
  providerId: string
  modelId: string
}
export interface ClipModelEligibility {
  ref: ClipModelRef
  status: ClipEligibilityStatus
}

export function isClipEligibilityStatus(value: string): value is ClipEligibilityStatus {
  return (CLIP_ELIGIBILITY_STATUSES as readonly string[]).includes(value)
}

const key = (ref: ClipModelRef) => `${ref.providerId} ${ref.modelId}`

/** The status of one model by EXACT provider/model ref. A model the answer does not name, or
 *  names twice, is unresolved: `undefined`, which no caller may read as eligible. */
export function clipEligibilityOf(
  list: readonly ClipModelEligibility[],
  ref: ClipModelRef,
): ClipEligibilityStatus | undefined {
  let found: ClipEligibilityStatus | undefined
  let seen = 0
  const wanted = key(ref)
  for (const item of list) {
    if (key(item.ref) !== wanted) continue
    seen++
    found = item.status
  }
  return seen === 1 ? found : undefined
}
