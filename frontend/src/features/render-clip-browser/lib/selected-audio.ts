import type { SelectedAudioRange, PcmChannels } from '@/shared/lib'

/** Only the bounded guard window is resampled. Its absolute common-lattice
 * origin keeps 44.1k/48k windows in the same phase as a whole-source decode. */
export async function canonicalSelectedAudio(
  range: SelectedAudioRange,
  startSample: number,
  frames: number,
  sampleRate: number,
  signal: AbortSignal,
): Promise<PcmChannels> {
  signal.throwIfAborted()
  const { metadata } = range
  if (metadata.channels !== 1 && metadata.channels !== 2)
    throw new Error('CLIP_SOURCE_AUDIO_CHANNELS_UNSUPPORTED')
  const outputFrames = Math.ceil((range.channels[0].length * sampleRate) / metadata.sampleRate)
  const offset = startSample - (range.startSample * sampleRate) / metadata.sampleRate
  if (
    !Number.isSafeInteger(offset) ||
    offset < 0 ||
    !Number.isSafeInteger(frames) ||
    frames <= 0 ||
    offset + frames > outputFrames
  )
    throw new Error('CLIP_SOURCE_AUDIO_TIMESTAMP_INVALID')
  if (metadata.sampleRate === sampleRate) {
    const gain = metadata.channels === 1 ? Math.SQRT1_2 : 1
    const channels = [0, 1].map((channel) => {
      const plane = new Float32Array(frames)
      const source = range.channels[metadata.channels === 1 ? 0 : channel]
      for (let n = 0; n < frames; n++) plane[n] = source[offset + n] * gain
      return plane
    })
    range.channels.length = 0
    return channels
  }
  const context = new OfflineAudioContext(2, outputFrames, sampleRate)
  const audio = context.createBuffer(
    metadata.channels,
    range.channels[0].length,
    metadata.sampleRate,
  )
  range.channels.forEach((channel, index) => audio.copyToChannel(channel, index))
  range.channels.length = 0
  const source = context.createBufferSource(),
    gain = context.createGain()
  source.buffer = audio
  gain.gain.value = metadata.channels === 1 ? Math.SQRT1_2 : 1
  source.connect(gain).connect(context.destination)
  source.start()
  try {
    const resampled = await context.startRendering()
    signal.throwIfAborted()
    return [0, 1].map((channel) => resampled.getChannelData(channel).slice(offset, offset + frames))
  } finally {
    source.disconnect()
    source.buffer = null
    gain.disconnect()
  }
}
