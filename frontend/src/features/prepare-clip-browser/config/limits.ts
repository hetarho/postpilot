/** Page-owned allocations and finite stage clocks; physical device memory is
 * measured independently during release qualification. */
export const ANALYSIS_PREPARATION_LIMITS = {
  sources: 20,
  durationMs: 30 * 60_000,
  copies: 49,
  intervalMs: 60_000,
  longEdge: 720,
  fps: 15,
  maxCopyBytes: 8 * 1024 * 1024,
  videoBitrates: [900_000, 650_000],
  audioBitrate: 64_000,
  audioRate: 48_000,
  maxFileBytes: 2 * 1024 * 1024 * 1024,
  maxReadBytes: 4 * 1024 * 1024,
  maxReadTotalBytes: 4 * 1024 * 1024 * 1024,
  maxCacheBytes: 1024 * 1024,
  timeoutMs: 30_000,
  operationTimeoutMs: 2 * 60 * 60_000,
  cleanupTimeoutMs: 1000,
  decoderReserveFrames: 48,
  decoderReserveBytes: 1024 * 1024 * 1024,
  maxDimension: 8192,
} as const

export const ANALYSIS_AUDIO_LIMITS = {
  maxFileBytes: ANALYSIS_PREPARATION_LIMITS.maxFileBytes,
  maxReadBytes: ANALYSIS_PREPARATION_LIMITS.maxReadBytes,
  maxReadTotalBytes: ANALYSIS_PREPARATION_LIMITS.maxReadTotalBytes,
  maxCacheBytes: ANALYSIS_PREPARATION_LIMITS.maxCacheBytes,
  timeoutMs: ANALYSIS_PREPARATION_LIMITS.timeoutMs,
  cleanupTimeoutMs: ANALYSIS_PREPARATION_LIMITS.cleanupTimeoutMs,
  maxPcmBytes: 128 * 1024 * 1024,
  maxChannels: 2,
  maxSampleRate: 192000,
  maxSampleFrames: 8192,
  decoderPrerollMs: 125,
  decoderTailMs: 125,
  decoderReserveSamples: 48,
  operationTimeoutMs: 60000,
} as const

export const ANALYSIS_AUDIO_ENCODER_LIMITS = {
  maxPcmBytes: 128 * 1024 * 1024,
  maxEncodedBytes: 1024 * 1024,
  maxEncodedPackets: 4096,
  maxPacketBytes: 64 * 1024,
  maxPrimingFrames: 4096,
  operationTimeoutMs: 60000,
  cleanupTimeoutMs: 1000,
} as const
