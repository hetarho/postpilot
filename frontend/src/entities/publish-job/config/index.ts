/** Publishing has its own durable queue and a shorter live-status projection. This is a
 *  public display/polling hint only; server leases remain authoritative. */
export const PUBLISH_JOB_POLL_MS = 2_000
