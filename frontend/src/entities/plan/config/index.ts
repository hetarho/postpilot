/** How long a credit balance is trusted before it is re-asked. The shell's credit control
 *  and the account popover read one cache entry, so a window is what keeps opening the
 *  popover from costing a second request; a generation takes far longer than this, so the
 *  figure is never stale by the time a job has actually moved it. */
export const PLAN_BALANCE_STALE_MS = 30_000
