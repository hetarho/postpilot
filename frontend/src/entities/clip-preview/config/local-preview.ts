/** Preview-only capacity. Export remains explicit full-quality30fps with no dropped frames. */
export const CLIP_LOCAL_PREVIEW = {
  scale: 0.5,
  cadence: 15,
  operationTimeoutMs: 30000,
  sampleEntries: 64,
  sampleBytes: 16 * 1024 * 1024,
} as const
