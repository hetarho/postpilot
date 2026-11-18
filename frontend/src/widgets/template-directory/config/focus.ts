/** How long a removal's focus handoff waits for its directory to mount. The reading navigates to
 *  the directory at once, so a stale handoff only means the owner went somewhere else first. */
export const REMOVAL_FOCUS_HANDOFF_TTL_MS = 5000
