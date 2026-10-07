/** Explicit page-owned budgets, not a claim about the browser's physical memory. */
export const CLIP_VIDEO_DECODING = {
  activeCuts: 2,
  presentationFrames: 2,
  presentationBytes: 64 * 1024 * 1024,
  // Locked Mediabunny1.58.0 starts with40 queued decode packets, then8.
  // Reserve both before opening a decoder, even though these are not all
  // necessarily live decoded frames. Actual device peaks remain T604 evidence.
  decoderReserveFrames: 48,
  decoderReserveBytes: 1024 * 1024 * 1024,
  bytesPerPixelReserve: 4,
  maxFileBytes: 2 * 1024 * 1024 * 1024,
  maxReadBytes: 4 * 1024 * 1024,
  maxReadTotalBytes: 512 * 1024 * 1024,
  maxCacheBytes: 1024 * 1024,
  timeoutMs: 30_000,
  workerCleanupMs: 1000,
} as const
