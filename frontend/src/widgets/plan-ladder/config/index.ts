/** The plan ladder's own motion tuning (ARCH-21): only this widget renders it. */

/** How far apart the plan ladder's rungs arrive, in milliseconds. Each rung's rise animation
 *  starts this much after the one before it, so four cards read as one ladder unfolding rather
 *  than four things appearing at once — and short enough that the last rung is still up before
 *  a reader has finished the first. */
export const PROMO_RISE_STAGGER_MS = 60

/** How long a plan card's post count takes to climb to a new value as a slider moves. Long
 *  enough to read as a count rather than a flicker, short enough to have settled before the
 *  thumb reaches the next slider stop. */
export const PROMO_COUNT_UP_MS = 480
