import {
  AudioSample,
  AudioSampleSource,
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
  type InputAudioTrack,
} from 'mediabunny'
import { bindNativeSourceColor, nativeSourceColorSpace } from './video/source-color'
import {
  createFiniteMediaSource,
  type BrowserMediaSourceAccess,
  type MediaRangeLimits,
} from './video/range-source'
import { openOriginalVideo } from './video/range-video'

export interface FiniteMediaCopyProfile extends MediaRangeLimits {
  durationMs: number
  decoderReserveFrames: number
  decoderReserveBytes: number
  maxCopyBytes: number
  fps: number
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
    audioSamples: number
    audioStartMs: number
    audioEndMs: number
    width: number
    height: number
    rotation: number
    hasAudio: boolean
    bitrate: number
  }
}

/** Full-original source time and cadence are measured separately from the copy
 * encoder. Only one decoded sample is retained by this consumer. */
export async function measureOriginalMedia(
  access: BrowserMediaSourceAccess,
  limits: FiniteMediaCopyProfile,
  signal: AbortSignal,
) {
  const video = await openOriginalVideo(access, limits, signal)
  let audioInput: Input | undefined
  let audioReader: ReturnType<typeof createFiniteMediaSource> | undefined
  try {
    const metadata = video.metadata
    if (
      metadata.codedWidth * metadata.codedHeight * 4 * limits.decoderReserveFrames >
      limits.decoderReserveBytes
    )
      throw new Error('CLIP_SOURCE_MEMORY_LIMIT')
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
    audioReader = createFiniteMediaSource(access, limits, signal)
    audioInput = new Input({ source: audioReader.source, formats: [MP4, QTFF, WEBM, MATROSKA] })
    const audio = (await audioInput.getAudioTracks())[0]
    if (audio && !(await audio.canDecode())) throw new Error('CLIP_SOURCE_AUDIO_CODEC_UNSUPPORTED')
    const audioRate = audio ? await audio.getSampleRate() : 0
    const audioChannels = audio ? await audio.getNumberOfChannels() : 0
    if (
      audio &&
      (audioChannels < 1 || audioChannels > 16 || audioRate < 8000 || audioRate > 192000)
    )
      throw new Error('CLIP_SOURCE_AUDIO_FORMAT_UNSUPPORTED')
    const gcd = (a: number, b: number): number => (b ? gcd(b, a % b) : a)
    const step = cadence ?? Math.max(1, Math.round((metadata.timeResolution * end) / frames))
    const divisor = gcd(metadata.timeResolution, step)
    return {
      durationMs,
      width: metadata.displayWidth,
      height: metadata.displayHeight,
      frameRateNumerator: metadata.timeResolution / divisor,
      frameRateDenominator: step / divisor,
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
          throw new Error('CLIP_ANALYSIS_COPY_COVERAGE')
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
    if (audio)
      for await (const sample of new AudioSampleSink(audio).samples()) {
        try {
          signal.throwIfAborted()
          if (audioSamples && Math.abs(sample.timestamp * 1000 - audioEndMs) > 0.05)
            throw new Error('CLIP_ANALYSIS_COPY_COVERAGE')
          audioSamples += sample.numberOfFrames
          if (audioSamples > limits.audioRate * 60 + 1024)
            throw new Error('CLIP_ANALYSIS_COPY_COVERAGE')
          audioStartMs = Math.min(audioStartMs, sample.timestamp * 1000)
          audioEndMs = Math.max(audioEndMs, (sample.timestamp + sample.duration) * 1000)
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
        audioSamples,
        audioStartMs: audio ? audioStartMs : 0,
        audioEndMs,
        width: await video.getDisplayWidth(),
        height: await video.getDisplayHeight(),
        rotation: await video.getRotation(),
        hasAudio: !!audio,
        bitrate,
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
      p.videoEndMs > 60000 + 0.01 ||
      Math.abs(p.videoEndMs - slot.durationMs) > tolerance ||
      (slot.hasAudio &&
        (Math.abs(p.audioStartMs) > 22 ||
          Math.abs(p.audioEndMs - slot.durationMs) > 22 ||
          Math.abs(p.audioSamples - (slot.durationMs * limits.audioRate) / 1000) > 1024))
    )
      throw new Error('CLIP_ANALYSIS_COPY_COVERAGE')
    return artifact
  } finally {
    signal.removeEventListener('abort', abort)
    input.dispose()
  }
}

/** Preserve the original common source clock, including silence before delayed
 * audio and at the tail. Trim on its integer sample lattice before resampling;
 * MP4 timestamp metadata alone cannot prove complete audible coverage. */
async function feedAnalysisAudio(
  track: InputAudioTrack,
  source: AudioSampleSource,
  slot: MediaCopyInterval,
  signal: AbortSignal,
) {
  const sampleRate = await track.getSampleRate(),
    channels = await track.getNumberOfChannels()
  const total = Math.round((slot.durationMs * sampleRate) / 1000)
  let cursor = 0
  async function silence(end: number) {
    while (cursor < end) {
      signal.throwIfAborted()
      const frames = Math.min(4096, end - cursor)
      const sample = new AudioSample({
        data: new Float32Array(frames * channels),
        format: 'f32-planar',
        numberOfChannels: channels,
        sampleRate,
        timestamp: cursor / sampleRate,
      })
      try {
        await source.add(sample)
      } finally {
        sample.close()
      }
      cursor += frames
    }
  }
  for await (const sample of new AudioSampleSink(track).samples(
    slot.offsetMs / 1000,
    (slot.offsetMs + slot.durationMs) / 1000,
  )) {
    try {
      signal.throwIfAborted()
      if (
        sample.sampleRate !== sampleRate ||
        sample.numberOfChannels !== channels ||
        sample.numberOfFrames * channels * 4 > 1024 * 1024
      )
        throw new Error('CLIP_SOURCE_AUDIO_MEMORY_LIMIT')
      const offset = Math.round((sample.timestamp - slot.offsetMs / 1000) * sampleRate)
      const begin = Math.max(0, cursor - offset),
        end = Math.min(sample.numberOfFrames, total - offset)
      if (end <= begin) continue
      await silence(Math.max(cursor, Math.min(total, offset)))
      const selected = sample.trim(begin, end)
      try {
        selected.setTimestamp(cursor / sampleRate)
        await source.add(selected)
        cursor += selected.numberOfFrames
      } finally {
        selected.close()
      }
    } finally {
      sample.close()
    }
  }
  await silence(total)
  source.close()
}

export async function transcodeMediaInterval(
  access: BrowserMediaSourceAccess,
  slot: MediaCopyInterval,
  limits: FiniteMediaCopyProfile,
  signal: AbortSignal,
  progress: (fraction: number) => void = () => {},
) {
  for (const bitrate of limits.videoBitrates) {
    signal.throwIfAborted()
    const reader = createFiniteMediaSource(access, limits, signal)
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
        tags: {},
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
      let audioSource: AudioSampleSource | undefined
      if (slot.hasAudio && audio) {
        audioSource = new AudioSampleSource({
          codec: 'aac',
          quality: new Quality({ bitrate: limits.audioBitrate }),
          transform: { sampleRate: limits.audioRate, numberOfChannels: 1 },
        })
        output.addAudioTrack(audioSource)
      }
      output.setMetadataTags({})
      await output.start()
      await Promise.all([
        conversion.execute(),
        audioSource && audio
          ? feedAnalysisAudio(audio, audioSource, slot, signal)
          : Promise.resolve(),
      ])
      await output.finalize()
      signal.throwIfAborted()
      return await inspectCompletedCopy(target.result(), slot, bitrate, limits, signal)
    } catch (error) {
      signal.throwIfAborted()
      if (
        !(error instanceof Error) ||
        error.message !== 'CLIP_ANALYSIS_COPY_TOO_LARGE' ||
        bitrate === limits.videoBitrates.at(-1)
      )
        throw error
    } finally {
      signal.removeEventListener('abort', abort)
      await conversion?.cancel().catch(() => undefined)
      await output?.cancel().catch(() => undefined)
      input.dispose()
      reader.dispose()
    }
  }
  throw new Error('CLIP_ANALYSIS_COPY_TOO_LARGE')
}
