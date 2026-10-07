import { integratedLoudness48k, truePeak48k } from './audio/loudness'
/** Decode finalized AAC with warmup, then measure its exact presentation window.
 * Final-output encoder bounds are independent of analysis-copy profiles. */
export async function verifyMp4Audio(
  file: Blob,
  expected: {
    sampleFrames: number
    sampleRate: number
    channels: number
    maxBytes: number
    paddingFrames: number
    maxSampleFrames: number
  },
  signal: AbortSignal,
) {
  signal.throwIfAborted()
  const { Input, BlobSource, MP4, AudioSampleSink } = await import('mediabunny')
  if (
    expected.sampleRate !== 48000 ||
    expected.channels !== 2 ||
    !Number.isSafeInteger(expected.sampleFrames) ||
    expected.sampleFrames <= 0 ||
    expected.sampleFrames * (expected.channels * 4 + 1) > expected.maxBytes
  )
    throw new Error('MEDIA_OUTPUT_AUDIO_MEMORY_LIMIT')
  const input = new Input({ source: new BlobSource(file), formats: [MP4] })
  const abort = () => input.dispose()
  signal.addEventListener('abort', abort, { once: true })
  const pcm = Array.from(
    { length: expected.channels },
    () => new Float32Array(expected.sampleFrames),
  )
  const coverage = new Uint8Array(expected.sampleFrames)
  let decodedFrames = 0,
    audibleFrames = 0,
    first = Infinity,
    last = -Infinity
  try {
    const track = await input.getPrimaryAudioTrack()
    if (!track) throw new Error('MEDIA_OUTPUT_AUDIO_MISSING')
    const config = await track.getDecoderConfig()
    if (
      config?.codec !== 'mp4a.40.2' ||
      config.sampleRate !== expected.sampleRate ||
      config.numberOfChannels !== expected.channels ||
      Math.abs((await track.computeDuration()) * expected.sampleRate - expected.sampleFrames) > 1
    )
      throw new Error('MEDIA_OUTPUT_AUDIO_TRACK_INVALID')
    for await (const sample of new AudioSampleSink(track).samples()) {
      try {
        signal.throwIfAborted()
        const start = Math.round(sample.timestamp * expected.sampleRate),
          end = start + sample.numberOfFrames
        decodedFrames += sample.numberOfFrames
        if (
          sample.sampleRate !== expected.sampleRate ||
          sample.numberOfChannels !== expected.channels ||
          sample.numberOfFrames > expected.maxSampleFrames ||
          start < -expected.paddingFrames ||
          end > expected.sampleFrames + expected.paddingFrames ||
          decodedFrames > expected.sampleFrames + 2 * expected.paddingFrames
        )
          throw new Error('MEDIA_OUTPUT_AUDIO_DECODE_LIMIT')
        first = Math.min(first, start)
        last = Math.max(last, end)
        const from = Math.max(0, start),
          to = Math.min(expected.sampleFrames, end)
        if (to <= from) continue
        for (let channel = 0; channel < expected.channels; channel++) {
          const buffer = new Float32Array(sample.numberOfFrames)
          sample.copyTo(buffer, { format: 'f32-planar', planeIndex: channel })
          pcm[channel]!.set(buffer.subarray(from - start, to - start), from)
        }
        for (let frame = from; frame < to; frame++) {
          if (coverage[frame]) throw new Error('MEDIA_OUTPUT_AUDIO_OVERLAP')
          coverage[frame] = 1
          audibleFrames++
        }
      } finally {
        sample.close()
      }
    }
    signal.throwIfAborted()
    if (audibleFrames !== expected.sampleFrames) throw new Error('MEDIA_OUTPUT_AUDIO_TRUNCATED')
    const loudness = integratedLoudness48k(pcm)
    return {
      sampleFrames: audibleFrames,
      decodedFrames,
      firstSample: first,
      lastSample: last,
      loudnessLUFS: Number.isFinite(loudness) ? loudness : undefined,
      truePeakDBTP: truePeak48k(pcm),
      silent: !Number.isFinite(loudness),
    }
  } finally {
    signal.removeEventListener('abort', abort)
    input.dispose()
  }
}
