import { Input, MP4, QTFF, WEBM, MATROSKA, AudioSampleSink } from 'mediabunny'
import {
  createFiniteMediaSource,
  MediaRangeError,
  type BrowserMediaSourceAccess,
  type MediaRangeLimits,
  type MediaRangePorts,
} from '../video/range-source'
import type { PcmChannels } from './processing-types'

export interface AudioRangeLimits extends MediaRangeLimits {
  maxPcmBytes: number
  maxChannels: number
  maxSampleRate: number
  maxSampleFrames: number
  decoderPrerollMs: number
  decoderTailMs: number
  decoderReserveSamples: number
  operationTimeoutMs: number
  cleanupTimeoutMs: number
}
export interface AudioSourceRange {
  startUs: number
  endUs: number
  targetSampleRate: number
}
export interface OriginalAudioMetadata {
  provenance: 'browser_client'
  codec: string
  streamNumber: number
  sampleRate: number
  channels: number
  firstTimestamp: number
}
export interface SelectedAudioRange {
  metadata: OriginalAudioMetadata
  /** Native PCM guard starts at this absolute input-sample clock. */
  startSample: number
  decodeStart: number
  channels: PcmChannels
  measurements: {
    sourceReads: number
    sourceBytes: number
    sourceReadMs: number
    openMs: number
    decodeMs: number
    decodedSamples: number
    logicalPcmBytes: number
    decoderReservedBytes: number
    physicalPeakBytes: null
  }
}
function gcd(a: number, b: number): number {
  return b ? gcd(b, a % b) : a
}
/** Absolute rational resampler phase: guard anchors lie on both sample lattices. */
export function audioGuardWindow(
  range: AudioSourceRange,
  sampleRate: number,
  firstTimestamp: number,
  limits: Pick<AudioRangeLimits, 'decoderPrerollMs' | 'decoderTailMs'>,
) {
  if (
    ![range.startUs, range.endUs, range.targetSampleRate, sampleRate].every(Number.isSafeInteger) ||
    range.startUs < 0 ||
    range.endUs <= range.startUs ||
    range.targetSampleRate <= 0 ||
    sampleRate <= 0 ||
    !Number.isFinite(firstTimestamp)
  )
    throw new MediaRangeError('CLIP_SOURCE_AUDIO_TIMESTAMP_INVALID')
  const cycle = sampleRate / gcd(sampleRate, range.targetSampleRate)
  const startSample =
    Math.floor(
      (Math.max(0, range.startUs / 1_000_000 - limits.decoderPrerollMs / 1000) * sampleRate) /
        cycle,
    ) * cycle
  const endSample = Math.ceil((range.endUs / 1_000_000 + limits.decoderTailMs / 1000) * sampleRate)
  return {
    startSample,
    endSample,
    decodeStart: Math.max(
      firstTimestamp,
      Math.min(
        startSample / sampleRate,
        range.startUs / 1_000_000 - limits.decoderPrerollMs / 1000,
      ),
    ),
    decodeEnd: endSample / sampleRate,
  }
}

/** Selected original sound only. The first audio stream matches native0:a:0;
 * player defaults and proxy cadence have no authority here. */
export async function decodeOriginalAudioRange(
  access: BrowserMediaSourceAccess,
  range: AudioSourceRange,
  limits: AudioRangeLimits,
  signal: AbortSignal,
  ports: MediaRangePorts = {},
): Promise<SelectedAudioRange | undefined> {
  signal.throwIfAborted()
  let sourceReadMs = 0
  const opened = performance.now()
  const reader = createFiniteMediaSource(access, limits, signal, {
    ...ports,
    read: (bytes, elapsedMs) => {
      sourceReadMs += elapsedMs
      ports.read?.(bytes, elapsedMs)
    },
  })
  const input = new Input({ source: reader.source, formats: [MP4, QTFF, WEBM, MATROSKA] })
  const abort = () => input.dispose()
  signal.addEventListener('abort', abort, { once: true })
  let iterator: ReturnType<AudioSampleSink['samples']> | undefined
  try {
    const track = (await input.getAudioTracks())[0]
    if (!track) return undefined
    if (await track.isLive()) throw new MediaRangeError('CLIP_SOURCE_RANGE_UNSUPPORTED')
    const [sampleRate, channels, firstTimestamp, codec] = await Promise.all([
      track.getSampleRate(),
      track.getNumberOfChannels(),
      track.getFirstTimestamp(),
      track.getCodec(),
    ])
    if (!Number.isSafeInteger(channels) || channels < 1 || channels > limits.maxChannels)
      throw new MediaRangeError('CLIP_SOURCE_AUDIO_CHANNELS_UNSUPPORTED')
    if (!Number.isSafeInteger(sampleRate) || sampleRate <= 0 || sampleRate > limits.maxSampleRate)
      throw new MediaRangeError('CLIP_SOURCE_AUDIO_RATE_UNSUPPORTED')
    // Bounded block profiles for the locked adapter. Codec-private allocations
    // remain unmeasured. PCM/ALAC/FLAC need a separately qualified block layout
    // before they can gain admission to this finite reservation.
    const codecFrameSamples: Readonly<Record<string, number>> = {
      aac: 2048,
      opus: 5760,
      mp3: 1152,
      vorbis: 8192,
    }
    const reservedSampleFrames = codecFrameSamples[codec ?? '']
    if (!reservedSampleFrames || reservedSampleFrames > limits.maxSampleFrames)
      throw new MediaRangeError('CLIP_SOURCE_AUDIO_BLOCK_UNSUPPORTED')
    if (!(await track.canDecode())) throw new MediaRangeError('CLIP_SOURCE_AUDIO_CODEC_UNSUPPORTED')
    const window = audioGuardWindow(range, sampleRate, firstTimestamp, limits)
    const openMs = performance.now() - opened
    const frames = window.endSample - window.startSample
    const logicalPcmBytes = frames * channels * Float32Array.BYTES_PER_ELEMENT
    const decoderReservedBytes =
      limits.decoderReserveSamples *
      reservedSampleFrames *
      channels *
      Float32Array.BYTES_PER_ELEMENT
    if (
      !Number.isSafeInteger(logicalPcmBytes) ||
      logicalPcmBytes <= 0 ||
      logicalPcmBytes + decoderReservedBytes > limits.maxPcmBytes
    )
      throw new MediaRangeError('CLIP_SOURCE_AUDIO_MEMORY_LIMIT')
    signal.throwIfAborted()
    const planes = Array.from({ length: channels }, () => new Float32Array(frames))
    // A declared audio stream may begin after this source-time selection.
    // Native first_pts=0 contributes leading silence; it does not seek forward
    // and move that later sound into an earlier cut.
    if (window.decodeStart < window.decodeEnd)
      iterator = new AudioSampleSink(track).samples(window.decodeStart, window.decodeEnd)
    let decodeMs = 0,
      decodedSamples = 0
    for (;;) {
      signal.throwIfAborted()
      if (!iterator) break
      const began = performance.now()
      const result = await iterator.next()
      decodeMs += performance.now() - began
      if (result.done) break
      const sample = result.value
      try {
        signal.throwIfAborted()
        if (
          sample.sampleRate !== sampleRate ||
          sample.numberOfChannels !== channels ||
          !Number.isSafeInteger(sample.numberOfFrames) ||
          sample.numberOfFrames > reservedSampleFrames
        )
          throw new MediaRangeError('CLIP_SOURCE_AUDIO_FORMAT_UNSUPPORTED')
        const sampleStart = Math.round(sample.timestamp * sampleRate)
        if (!Number.isSafeInteger(sampleStart))
          throw new MediaRangeError('CLIP_SOURCE_AUDIO_TIMESTAMP_INVALID')
        const from = Math.max(sampleStart, window.startSample),
          to = Math.min(sampleStart + sample.numberOfFrames, window.endSample)
        if (to > from)
          for (let channel = 0; channel < channels; channel++)
            sample.copyTo(
              planes[channel].subarray(from - window.startSample, to - window.startSample),
              {
                format: 'f32-planar',
                planeIndex: channel,
                frameOffset: from - sampleStart,
                frameCount: to - from,
              },
            )
        decodedSamples += sample.numberOfFrames
      } finally {
        sample.close()
      }
    }
    signal.throwIfAborted()
    if (!decodedSamples && iterator) throw new MediaRangeError('CLIP_SOURCE_AUDIO_RANGE_EMPTY')
    const measurements = reader.measurements()
    return {
      metadata: {
        provenance: 'browser_client',
        codec: codec ?? 'unknown',
        streamNumber: track.number,
        sampleRate,
        channels,
        firstTimestamp,
      },
      startSample: window.startSample,
      decodeStart: window.decodeStart,
      channels: planes,
      measurements: {
        sourceReads: measurements.reads,
        sourceBytes: measurements.bytesRead,
        sourceReadMs,
        openMs,
        decodeMs,
        decodedSamples,
        logicalPcmBytes,
        decoderReservedBytes,
        physicalPeakBytes: null,
      },
    }
  } finally {
    signal.removeEventListener('abort', abort)
    input.dispose()
    reader.dispose()
    await iterator?.return()
  }
}
