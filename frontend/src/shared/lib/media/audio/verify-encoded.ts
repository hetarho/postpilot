import { audioEncoderDelay } from './encoder-delay'
import { integratedLoudness48k, truePeak48k } from './loudness'
import type { EncodedAudioTrack, PcmChannels } from './processing-types'

/** Measure the newly encoded track once; the muxer must discard this priming
 * and keep exactly sampleFrames of audible PCM via the track's edit window. */
export async function verifyEncodedAudio(
  chunks: EncodedAudioTrack['chunks'],
  config: AudioDecoderConfig,
  reference: PcmChannels,
) {
  const frames: { start: number; channels: PcmChannels }[] = []
  let length = 0,
    failure: DOMException | undefined
  const decoder = new AudioDecoder({
    output: (audio) => {
      try {
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
    },
  })
  try {
    decoder.configure(config)
    for (const chunk of chunks) decoder.decode(new EncodedAudioChunk(chunk))
    await decoder.flush()
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
      truePeakDBTP: truePeak48k(aligned),
      silent: !Number.isFinite(loudness),
    }
  } finally {
    if (decoder.state !== 'closed') decoder.close()
  }
}
