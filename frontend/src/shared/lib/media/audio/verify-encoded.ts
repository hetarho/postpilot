import { audioEncoderDelay } from './encoder-delay'
import { integratedLoudness48k, truePeak48k, loudnessRange48k } from './loudness'
import type { EncodedAudioTrack, PcmChannels, AudioProcessorLimits } from './processing-types'
import { waitAudioCodecCapacity, drainAudioCodec } from './codec-queue'

/** Measure the newly encoded track once; the muxer must discard this priming
 * and keep exactly sampleFrames of audible PCM via the track's edit window. */
export async function verifyEncodedAudio(
  chunks: EncodedAudioTrack['chunks'],
  config: AudioDecoderConfig,
  reference: PcmChannels,
  limits: AudioProcessorLimits,
  signal: AbortSignal,
  queueSize: number,
) {
  signal.throwIfAborted()
  const controller = new AbortController()
  const abort = () => controller.abort(signal.reason)
  signal.addEventListener('abort', abort, { once: true })
  const frames: { start: number; channels: PcmChannels }[] = []
  let length = 0,
    failure: DOMException | undefined
  const decoder = new AudioDecoder({
    output: (audio) => {
      try {
        if (controller.signal.aborted) return
        const next = length + audio.numberOfFrames
        if (
          audio.numberOfChannels !== reference.length ||
          audio.sampleRate !== config.sampleRate ||
          next > reference[0].length + limits.maxPrimingFrames ||
          next * reference.length * 4 * 2 > limits.maxPcmBytes
        ) {
          controller.abort(new Error('AUDIO_VERIFY_MEMORY_LIMIT'))
          return
        }
        const channels = Array.from({ length: audio.numberOfChannels }, (_, index) => {
          const samples = new Float32Array(audio.numberOfFrames)
          audio.copyTo(samples, { planeIndex: index, format: 'f32-planar' })
          return samples
        })
        frames.push({ start: length, channels })
        length += audio.numberOfFrames
      } finally {
        audio.close()
      }
    },
    error: (error) => {
      failure = error
      controller.abort(error)
    },
  })
  const close = () => {
    if (decoder.state !== 'closed') decoder.close()
  }
  controller.signal.addEventListener('abort', close, { once: true })
  try {
    decoder.configure(config)
    for (const chunk of chunks) {
      await waitAudioCodecCapacity(decoder, queueSize, controller.signal, limits.operationTimeoutMs)
      decoder.decode(new EncodedAudioChunk(chunk))
    }
    await drainAudioCodec(decoder, controller.signal, limits.operationTimeoutMs)
    if (failure) throw failure
    if (length < reference[0].length) throw new Error('AUDIO_TRACK_TRUNCATED')
    const decoded = reference.map(() => new Float32Array(length))
    for (const frame of frames)
      frame.channels.forEach((channel, index) => decoded[index].set(channel, frame.start))
    frames.length = 0
    const primingFrames = audioEncoderDelay(reference, decoded)
    const aligned = decoded.map((channel) =>
      channel.subarray(primingFrames, primingFrames + reference[0].length),
    )
    const loudness = integratedLoudness48k(aligned)
    return {
      primingFrames,
      loudnessLUFS: Number.isFinite(loudness) ? loudness : undefined,
      loudnessRangeLU: loudnessRange48k(aligned),
      truePeakDBTP: truePeak48k(aligned),
      silent: !Number.isFinite(loudness),
    }
  } finally {
    signal.removeEventListener('abort', abort)
    controller.signal.removeEventListener('abort', close)
    close()
    frames.length = 0
  }
}
