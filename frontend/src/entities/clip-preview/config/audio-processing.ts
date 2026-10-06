import { CLIP_BROWSER_RENDER } from '@/entities/clip-design/@x/clip-preview'
import { CLIP_VIDEO_DECODING } from './video-decoding'

/** Logical owned-memory admission, including overlap between mix/codec phases.
 * These are conservative reservations, not measured codec-private allocations. */
export const CLIP_AUDIO_PROCESSING = {
  maxFileBytes: CLIP_VIDEO_DECODING.maxFileBytes,
  maxReadBytes: CLIP_VIDEO_DECODING.maxReadBytes,
  maxReadTotalBytes: CLIP_VIDEO_DECODING.maxReadTotalBytes,
  maxCacheBytes: CLIP_VIDEO_DECODING.maxCacheBytes,
  timeoutMs: CLIP_VIDEO_DECODING.timeoutMs,
  maxPcmBytes: CLIP_BROWSER_RENDER.audioDecodedBytes,
  maxChannels: 2,
  maxSampleRate: 192_000,
  maxSampleFrames: 8192,
  decoderPrerollMs: 125,
  decoderTailMs: 125,
  decoderReserveSamples: 48,
  mixCopies: 6,
  speechCopies: 2,
  transientRangeCopies: 3,
  canonicalRangeCopies: 2,
  dspWorkingBytes: 4 * 1024 * 1024,
  encodedPacketBytes: 8 * 1024 * 1024,
  encodedPackets: 4096,
  encodedPacketMaxBytes: 64 * 1024,
  encoderPaddingFrames: 4096,
  operationTimeoutMs: 60_000,
  cleanupTimeoutMs: 1000,
} as const
