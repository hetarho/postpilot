import type { EncodedAudioTrack } from './audio/processing-types'

interface VideoTrack {
  config: VideoEncoderConfig
  decoderConfig: VideoDecoderConfig
  chunks: {
    data: Uint8Array<ArrayBuffer>
    type: EncodedVideoChunkType
    timestamp: number
    duration: number
  }[]
}

/** Mux existing WebCodecs packets. No encoding, local playback URL or persistence. */
export async function muxMp4(
  video: VideoTrack,
  audio: EncodedAudioTrack | undefined,
  signal: AbortSignal,
): Promise<Blob> {
  signal.throwIfAborted()
  const {
    BufferTarget,
    EncodedAudioPacketSource,
    EncodedPacket,
    EncodedVideoPacketSource,
    Mp4OutputFormat,
    Output,
  } = await import('mediabunny')
  signal.throwIfAborted()
  const target = new BufferTarget()
  const output = new Output({ target, format: new Mp4OutputFormat({ fastStart: 'in-memory' }) })
  const picture = new EncodedVideoPacketSource('avc')
  const sound = audio ? new EncodedAudioPacketSource('aac') : undefined
  output.addVideoTrack(picture, { frameRate: video.config.framerate })
  if (sound) output.addAudioTrack(sound)
  try {
    await output.start()
    for (const [i, chunk] of video.chunks.entries()) {
      signal.throwIfAborted()
      await picture.add(
        new EncodedPacket(chunk.data, chunk.type, chunk.timestamp / 1e6, chunk.duration / 1e6),
        i === 0 ? { decoderConfig: video.decoderConfig } : undefined,
      )
    }
    picture.close()
    if (audio && sound) {
      const rate = audio.config.sampleRate
      const end = audio.sampleFrames / rate
      const origin = audio.chunks[0]?.timestamp ?? 0
      for (const [i, chunk] of audio.chunks.entries()) {
        signal.throwIfAborted()
        // Negative presentation time becomes an MP4 edit list. Keep priming
        // packets for decoder warmup, trim only their presentation and tail padding.
        const start =
          Math.round(((chunk.timestamp - origin) * rate) / 1e6) / rate - audio.primingFrames / rate
        if (start >= end) break
        const duration = Math.min(Math.round((chunk.duration * rate) / 1e6) / rate, end - start)
        await sound.add(
          new EncodedPacket(chunk.data, chunk.type, start, duration),
          i === 0 ? { decoderConfig: audio.decoderConfig } : undefined,
        )
      }
      sound.close()
    }
    signal.throwIfAborted()
    await output.finalize()
    signal.throwIfAborted()
    if (!target.buffer) throw new Error('Missing MP4 output')
    return new Blob([target.buffer], { type: 'video/mp4' })
  } finally {
    if (output.state !== 'finalized') await output.cancel()
  }
}
