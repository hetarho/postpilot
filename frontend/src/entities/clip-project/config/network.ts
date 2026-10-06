/** Bound the response wait; an uncertain durable start is reconciled by reads
 * and never automatically retried. */
export const CLIP_START_RPC_TIMEOUT_MS = 30_000
