import {
  canEncodeAudio,
  canEncodeVideo,
  EncodedAudioPacketSource,
  EncodedPacket,
  AudioSampleSink,
  BlobSource,
  Conversion,
  Input,
  MP4,
  QTFF,
  WEBM,
  MATROSKA,
  Mp4OutputFormat,
  Output,
  Quality,
  StreamTarget,
  VideoSampleSink,
} from 'mediabunny'
import { bindNativeSourceColor, nativeSourceColorSpace } from './video/source-color'
import {
  createFiniteMediaSource,
  type BrowserMediaSourceAccess,
  type MediaRangeLimits,
} from './video/range-source'
import { explicitSquarePixelsMp4 } from './square-pixel-mp4'
import type { EncodedAudioTrack } from './audio/processing-types'
import { openOriginalVideo } from './video/range-video'

export interface FiniteMediaCopyProfile extends MediaRangeLimits {
  durationMs: number
  decoderReserveFrames: number
  decoderReserveBytes: number
  maxCopyBytes: number
  fps: number
  longEdge: number
  videoBitrates: readonly number[]
  audioBitrate: number
  audioRate: number
}
export interface MediaCopyInterval {
  offsetMs: number
  durationMs: number
  width: number
  height: number
  hasAudio: boolean
}
export interface CompletedMediaCopy {
  buffer: ArrayBuffer
  sha256: string
  inspection: {
    bytes: number
    videoFrames: number
    videoStartMs: number
    videoEndMs: number
    containerEndMs: number
    audioSamples: number
    audioStartMs: number
    audioEndMs: number
    width: number
    height: number
    rotation: number
    hasAudio: boolean
    targetVideoBitrate: number
    actualVideoBitrate: number
    actualAudioBitrate: number
  }
}

function coverageFailure(details: Record<string, unknown>): never {
  throw Object.assign(new Error('CLIP_ANALYSIS_COPY_COVERAGE'), { details })
}

function sourceLimits(limits: MediaRangeLimits): MediaRangeLimits {
  return {
    maxFileBytes: limits.maxFileBytes,
    maxReadBytes: limits.maxReadBytes,
    maxReadTotalBytes: limits.maxReadTotalBytes,
    maxCacheBytes: limits.maxCacheBytes,
    timeoutMs: limits.timeoutMs,
  }
}

/** Full-original source time and cadence are measured separately from the copy
 * encoder. Only one decoded sample is retained by this consumer. */
export async function measureOriginalMedia(
  access: BrowserMediaSourceAccess,
  limits: FiniteMediaCopyProfile,
  signal: AbortSignal,
) {
  const video = await openOriginalVideo(access, sourceLimits(limits), signal)
  let audioInput: Input | undefined
  let audioReader: ReturnType<typeof createFiniteMediaSource> | undefined
  try {
    const metadata = video.metadata
    if (
      metadata.codedWidth * metadata.codedHeight * 4 * limits.decoderReserveFrames >
      limits.decoderReserveBytes
    )
      throw new Error('CLIP_SOURCE_MEMORY_LIMIT')
    const scale = Math.min(
      1,
      limits.longEdge / Math.max(metadata.displayWidth, metadata.displayHeight),
    )
    if (
      !(await canEncodeVideo('avc', {
        width: Math.max(2, Math.floor((metadata.displayWidth * scale) / 2) * 2),
        height: Math.max(2, Math.floor((metadata.displayHeight * scale) / 2) * 2),
        frameRate: limits.fps,
        quality: new Quality({ bitrate: limits.videoBitrates[0], bitrateMode: 'constant' }),
      }))
    )
      throw new Error('CLIP_ANALYSIS_ENCODER_UNSUPPORTED')
    let frames = 0,
      end = 0,
      previous: number | undefined,
      cadence: number | undefined,
      constant = true
    for await (const sample of video.samples(0, limits.durationMs / 1000 + 1)) {
      try {
        signal.throwIfAborted()
        const ticks = Math.round(sample.timestamp * metadata.timeResolution)
        if (previous !== undefined) {
          const delta = ticks - previous
          if (delta <= 0) throw new Error('CLIP_SOURCE_TIMESTAMP_INVALID')
          if (cadence !== undefined && delta !== cadence) constant = false
          cadence ??= delta
        }
        previous = ticks
        frames++
        end = Math.max(end, sample.timestamp + sample.duration)
        if (!Number.isFinite(end) || end > limits.durationMs / 1000 + 0.022 || frames > 100_000_000)
          throw new Error('CLIP_INPUT_TOO_LARGE')
      } finally {
        sample.close()
      }
    }
    if (frames < 1 || end <= 0) throw new Error('CLIP_SOURCE_MEASUREMENT_INVALID')
    let durationMs = Math.round(end * 1000)
    // A container declaration may settle tiny edit-list tails, never conceal a
    // longer decoded timeline. Source cadence is integer original PTS evidence.
    const declared = metadata.durationFromMetadata
    if (declared !== null && Math.abs(declared * 1000 - durationMs) <= 22)
      durationMs = Math.round(declared * 1000)
    audioReader = createFiniteMediaSource(access, sourceLimits(limits), signal)
    audioInput = new Input({ source: audioReader.source, formats: [MP4, QTFF, WEBM, MATROSKA] })
    const audio = (await audioInput.getAudioTracks())[0]
    if (
      audio &&
      !(await canEncodeAudio('aac', {
        numberOfChannels: 1,
        sampleRate: limits.audioRate,
        quality: new Quality({ bitrate: limits.audioBitrate }),
      }))
    )
      throw new Error('CLIP_ANALYSIS_ENCODER_UNSUPPORTED')
    if (audio && !(await audio.canDecode())) throw new Error('CLIP_SOURCE_AUDIO_CODEC_UNSUPPORTED')
    const audioRate = audio ? await audio.getSampleRate() : 0
    const audioChannels = audio ? await audio.getNumberOfChannels() : 0
    if (
      audio &&
      (audioChannels < 1 || audioChannels > 16 || audioRate < 8000 || audioRate > 192000)
    )
      throw new Error('CLIP_SOURCE_AUDIO_FORMAT_UNSUPPORTED')
    const gcd = (a: number, b: number): number => (b ? gcd(b, a % b) : a)
    const numerator =
      constant && cadence ? metadata.timeResolution : frames * metadata.timeResolution
    const denominator = constant && cadence ? cadence : Math.round(end * metadata.timeResolution)
    const divisor = gcd(numerator, denominator)
    return {
      provenance: metadata.provenance,
      durationMs,
      width: metadata.displayWidth,
      height: metadata.displayHeight,
      frameRateNumerator: numerator / divisor,
      frameRateDenominator: denominator / divisor,
      cadenceVerified: frames > 1 && constant,
      decodedFrames: frames,
      hasAudio: !!audio,
      audioRate,
      audioChannels,
    }
  } finally {
    video.dispose()
    audioInput?.dispose()
    audioReader?.dispose()
  }
}

/** A finite random-access mux target. Crossing the cap aborts that attempt;
 * only a finalized, independently decoded second attempt may be handed off. */
export function boundedMediaCopyTarget(maxBytes: number) {
  const storage = new Uint8Array(maxBytes)
  let size = 0
  const target = new StreamTarget(
    new WritableStream({
      write(chunk) {
        const end = chunk.position + chunk.data.byteLength
        if (!Number.isSafeInteger(chunk.position) || chunk.position < 0 || end > maxBytes)
          throw new Error('CLIP_ANALYSIS_COPY_TOO_LARGE')
        storage.set(chunk.data, chunk.position)
        size = Math.max(size, end)
      },
    }),
    { chunked: false },
  )
  return { target, result: () => storage.slice(0, size).buffer }
}

async function inspectCompletedCopy(
  buffer: ArrayBuffer,
  slot: MediaCopyInterval,
  bitrate: number,
  limits: FiniteMediaCopyProfile,
  signal: AbortSignal,
  primingFrames = 0,
) {
  const input = new Input({ source: new BlobSource(new Blob([buffer])), formats: [MP4] })
  const abort = () => input.dispose()
  signal.addEventListener('abort', abort, { once: true })
  try {
    const videos = await input.getVideoTracks(),
      audios = await input.getAudioTracks()
    const video = videos[0],
      audio = audios[0]
    if (
      (await input.getTracks()).length !== 1 + Number(slot.hasAudio) ||
      videos.length !== 1 ||
      audios.length !== Number(slot.hasAudio) ||
      !video ||
      (await video.getCodec()) !== 'avc' ||
      (await video.getRotation()) !== 0 ||
      (video &&
        (await video.getPixelAspectRatio()).num !== (await video.getPixelAspectRatio()).den) ||
      (audio &&
        ((await audio.getCodec()) !== 'aac' ||
          (await audio.getSampleRate()) !== limits.audioRate ||
          (await audio.getNumberOfChannels()) !== 1))
    )
      throw new Error('CLIP_ANALYSIS_COPY_PROFILE')
    let videoFrames = 0,
      videoStartMs = Infinity,
      videoEndMs = 0,
      previous: number | undefined
    for await (const sample of new VideoSampleSink(video).samples()) {
      try {
        signal.throwIfAborted()
        if (
          videoFrames >= limits.fps * 60 ||
          (previous !== undefined &&
            Math.abs(sample.timestamp - previous - 1 / limits.fps) > 0.000002)
        )
          coverageFailure({
            phase: 'video',
            videoFrames,
            previous,
            timestamp: sample.timestamp,
            duration: sample.duration,
          })
        previous = sample.timestamp
        videoFrames++
        videoStartMs = Math.min(videoStartMs, sample.timestamp * 1000)
        videoEndMs = Math.max(videoEndMs, (sample.timestamp + sample.duration) * 1000)
      } finally {
        sample.close()
      }
    }
    let audioSamples = 0,
      audioStartMs = Infinity,
      audioEndMs = 0
    let rawSamples = 0,
      rawEnd: number | undefined
    const expectedAudioSamples = (slot.durationMs * limits.audioRate) / 1000
    if (audio)
      for await (const sample of new AudioSampleSink(audio).samples()) {
        try {
          signal.throwIfAborted()
          const sampleStart = Math.round(sample.timestamp * limits.audioRate)
          const sampleEnd = sampleStart + sample.numberOfFrames
          rawSamples += sample.numberOfFrames
          if (
            sample.sampleRate !== limits.audioRate ||
            sample.numberOfChannels !== 1 ||
            (rawEnd !== undefined && sampleStart !== rawEnd) ||
            sampleStart < -primingFrames - 1 ||
            sampleEnd > expectedAudioSamples + 1024 ||
            rawSamples > expectedAudioSamples + primingFrames + 1024
          )
            coverageFailure({
              phase: 'audioRaw',
              rawSamples,
              rawEnd,
              sampleStart,
              sampleEnd,
              primingFrames,
            })
          rawEnd = sampleEnd
          // Decoder warmup and packet padding are decoded through EOF, but the
          // playable edit window contributes only its exact source-time samples.
          const begin = Math.max(0, sampleStart),
            end = Math.min(expectedAudioSamples, sampleEnd)
          if (end > begin) {
            if (begin !== audioSamples) coverageFailure({ phase: 'audioGap', begin, audioSamples })
            audioSamples += end - begin
            audioStartMs = Math.min(audioStartMs, (begin * 1000) / limits.audioRate)
            audioEndMs = (end * 1000) / limits.audioRate
          }
        } finally {
          sample.close()
        }
      }
    const digest = await crypto.subtle.digest('SHA-256', buffer)
    const sha256 = Array.from(new Uint8Array(digest), (n) => n.toString(16).padStart(2, '0')).join(
      '',
    )
    const artifact: CompletedMediaCopy = {
      buffer,
      sha256,
      inspection: {
        bytes: buffer.byteLength,
        videoFrames,
        videoStartMs,
        videoEndMs,
        containerEndMs: ((await input.getDurationFromMetadata()) ?? NaN) * 1000,
        audioSamples,
        audioStartMs: audio ? audioStartMs : 0,
        audioEndMs,
        width: await video.getDisplayWidth(),
        height: await video.getDisplayHeight(),
        rotation: await video.getRotation(),
        hasAudio: !!audio,
        targetVideoBitrate: bitrate,
        actualVideoBitrate: (await video.computePacketStats()).averageBitrate,
        actualAudioBitrate: audio ? (await audio.computePacketStats()).averageBitrate : 0,
      },
    }
    const p = artifact.inspection,
      tolerance = 1000 / limits.fps + 0.01
    if (
      !Object.values(p)
        .filter((value): value is number => typeof value === 'number')
        .every(Number.isFinite) ||
      buffer.byteLength < 1 ||
      buffer.byteLength > limits.maxCopyBytes ||
      p.width !== slot.width ||
      p.height !== slot.height ||
      p.rotation !== 0 ||
      p.videoFrames < 1 ||
      p.videoFrames > limits.fps * 60 ||
      Math.abs((p.videoFrames * 1000) / limits.fps - slot.durationMs) > tolerance ||
      Math.abs(p.videoStartMs) > 0.01 ||
      p.containerEndMs > 60000 + 0.01 ||
      Math.abs(p.containerEndMs - slot.durationMs) > tolerance ||
      p.videoEndMs > 60000 + 0.01 ||
      Math.abs(p.videoEndMs - slot.durationMs) > tolerance ||
      (slot.hasAudio &&
        (Math.abs(p.audioStartMs) > 22 ||
          Math.abs(p.audioEndMs - slot.durationMs) > 22 ||
          Math.abs(p.audioSamples - (slot.durationMs * limits.audioRate) / 1000) > 1024))
    )
      coverageFailure({ phase: 'completed', expectedMs: slot.durationMs, ...p })
    return artifact
  } finally {
    signal.removeEventListener('abort', abort)
    input.dispose()
  }
}

/** Retain decoder warmup packets, then expose exactly the measured source-time
 * samples through a standard MP4 edit window. No real audible tail is removed. */
async function feedCopyAudio(
  audio: EncodedAudioTrack,
  source: EncodedAudioPacketSource,
  signal: AbortSignal,
) {
  const rate = audio.config.sampleRate,
    end = audio.sampleFrames / rate
  const origin = audio.chunks[0]?.timestamp ?? 0
  for (const [index, chunk] of audio.chunks.entries()) {
    signal.throwIfAborted()
    const start =
      Math.round(((chunk.timestamp - origin) * rate) / 1e6) / rate - audio.primingFrames / rate
    if (start >= end) break
    const duration = Math.min(Math.round((chunk.duration * rate) / 1e6) / rate, end - start)
    await source.add(
      new EncodedPacket(chunk.data, chunk.type, start, duration),
      index === 0 ? { decoderConfig: audio.decoderConfig } : undefined,
    )
  }
  source.close()
}

/** Retry only a failed byte-bound attempt. Every operation must finish its
 * cleanup before a lower bitrate may start; all other failures are terminal. */
export async function runMediaSizeAttempts<T>(
  bitrates: readonly number[],
  signal: AbortSignal,
  attempt: (bitrate: number) => Promise<T>,
): Promise<T> {
  if (!bitrates.length || bitrates.length > 2) throw new Error('CLIP_ANALYSIS_QUEUE_LIMIT')
  for (const bitrate of bitrates) {
    signal.throwIfAborted()
    try {
      return await attempt(bitrate)
    } catch (error) {
      signal.throwIfAborted()
      if (
        !(error instanceof Error) ||
        error.message !== 'CLIP_ANALYSIS_COPY_TOO_LARGE' ||
        bitrate === bitrates.at(-1)
      )
        throw error
    }
  }
  throw new Error('CLIP_ANALYSIS_COPY_TOO_LARGE')
}

export async function transcodeMediaInterval(
  access: BrowserMediaSourceAccess,
  slot: MediaCopyInterval,
  limits: FiniteMediaCopyProfile,
  signal: AbortSignal,
  progress: (fraction: number) => void = () => {},
  encodedAudio?: EncodedAudioTrack,
) {
  if (
    slot.hasAudio &&
    (!encodedAudio ||
      encodedAudio.sampleFrames !== (slot.durationMs * limits.audioRate) / 1000 ||
      encodedAudio.config.numberOfChannels !== 1 ||
      encodedAudio.config.sampleRate !== limits.audioRate)
  )
    throw new Error('CLIP_SOURCE_AUDIO_TIMESTAMP_INVALID')
  return runMediaSizeAttempts(limits.videoBitrates, signal, async (bitrate) => {
    signal.throwIfAborted()
    const reader = createFiniteMediaSource(access, sourceLimits(limits), signal)
    const input = new Input({ source: reader.source, formats: [MP4, QTFF, WEBM, MATROSKA] })
    let conversion: Conversion | undefined
    let output: Output | undefined
    const abort = () => {
      void conversion?.cancel()
      void output?.cancel()
      input.dispose()
    }
    signal.addEventListener('abort', abort, { once: true })
    try {
      const video = (await input.getVideoTracks())[0],
        audio = (await input.getAudioTracks())[0]
      if (!video || (await video.computePacketStats(2)).packetCount < 2)
        throw new Error('CLIP_SOURCE_STREAM_AMBIGUOUS')
      if (
        (await video.getCodedWidth()) *
          (await video.getCodedHeight()) *
          4 *
          limits.decoderReserveFrames >
        limits.decoderReserveBytes
      )
        throw new Error('CLIP_SOURCE_MEMORY_LIMIT')
      bindNativeSourceColor(video, nativeSourceColorSpace(await video.getColorSpace()))
      // The native copy strips descriptive source metadata. Keep no original
      // stream name or language tag beyond the footage/audio itself.
      video.getName = async () => null
      video.getLanguageCode = async () => 'und'
      const target = boundedMediaCopyTarget(limits.maxCopyBytes)
      output = new Output({
        format: new Mp4OutputFormat({ fastStart: false }),
        target: target.target,
      })
      conversion = await Conversion.init({
        input,
        output,
        tracks: 'all',
        composable: true,
        copy: false,
        trim: { start: slot.offsetMs / 1000, end: (slot.offsetMs + slot.durationMs) / 1000 },
        video: (track) =>
          track === video
            ? {
                codec: 'avc',
                quality: new Quality({ bitrate, bitrateMode: 'constant' }),
                width: slot.width,
                height: slot.height,
                fit: 'fill',
                frameRate: limits.fps,
                allowTransformationMetadata: false,
                keyFrameInterval: 5,
                forceTranscode: true,
              }
            : { discard: true },
        audio: { discard: true },
        showWarnings: false,
      })
      if (
        !conversion.isValid ||
        !conversion.utilizedTracks.includes(video) ||
        (slot.hasAudio && (!audio || !(await audio.canDecode())))
      )
        throw new Error('CLIP_ANALYSIS_ENCODER_UNSUPPORTED')
      signal.throwIfAborted()
      conversion.onProgress = progress
      let audioSource: EncodedAudioPacketSource | undefined
      if (slot.hasAudio && audio) {
        audioSource = new EncodedAudioPacketSource('aac')
        output.addAudioTrack(audioSource)
      }
      output.setMetadataTags({})
      await output.start()
      await Promise.all([
        conversion.execute(),
        audioSource && encodedAudio
          ? feedCopyAudio(encodedAudio, audioSource, signal)
          : Promise.resolve(),
      ])
      await output.finalize()
      signal.throwIfAborted()
      return await inspectCompletedCopy(
        explicitSquarePixelsMp4(target.result(), limits.maxCopyBytes),
        slot,
        bitrate,
        limits,
        signal,
        encodedAudio?.primingFrames ?? 0,
      )
    } finally {
      signal.removeEventListener('abort', abort)
      await conversion?.cancel().catch(() => undefined)
      if (output?.state !== 'finalized') await output?.cancel().catch(() => undefined)
      input.dispose()
      reader.dispose()
    }
  })
}
