import type { BoundedMediaOutput } from './bounded-output'
import type { EncodedAudioTrack } from './audio/processing-types'

export interface MediaEncodedVideoPacket {
  type: EncodedVideoChunkType
  timestamp: number
  duration: number
  data: Uint8Array<ArrayBuffer>
}
/** One standard seekable MP4. Packet admission propagates the output stream's pressure. */
export async function createMp4PacketMux(
  target: BoundedMediaOutput,
  frameRate: number,
  audio: EncodedAudioTrack | undefined,
  signal: AbortSignal,
) {
  const {
    Output,
    StreamTarget,
    Mp4OutputFormat,
    EncodedVideoPacketSource,
    EncodedAudioPacketSource,
    EncodedPacket,
  } = await import('mediabunny')
  signal.throwIfAborted()
  const output = new Output({
    target: new StreamTarget(target.stream),
    format: new Mp4OutputFormat({ fastStart: false }),
  })
  const picture = new EncodedVideoPacketSource('avc')
  const sound = audio ? new EncodedAudioPacketSource('aac') : undefined
  output.addVideoTrack(picture, { frameRate })
  if (sound) output.addAudioTrack(sound)
  await output.start()
  let closed = false,
    count = 0
  return {
    async video(packet: MediaEncodedVideoPacket, decoderConfig?: VideoDecoderConfig) {
      signal.throwIfAborted()
      if (closed) throw new Error('MEDIA_MUX_CLOSED')
      await picture.add(
        new EncodedPacket(packet.data, packet.type, packet.timestamp / 1e6, packet.duration / 1e6),
        decoderConfig ? { decoderConfig } : undefined,
      )
      count++
      signal.throwIfAborted()
    },
    async finalize() {
      signal.throwIfAborted()
      closed = true
      picture.close()
      if (audio && sound) {
        const rate = audio.config.sampleRate,
          end = audio.sampleFrames / rate
        const origin = audio.chunks[0]?.timestamp ?? 0
        for (const [i, chunk] of audio.chunks.entries()) {
          signal.throwIfAborted()
          // Preserve the priming edit list and exact selected PCM hard end.
          const start =
            Math.round(((chunk.timestamp - origin) * rate) / 1e6) / rate -
            audio.primingFrames / rate
          if (start >= end) break
          const duration = Math.min(Math.round((chunk.duration * rate) / 1e6) / rate, end - start)
          await sound.add(
            new EncodedPacket(chunk.data, chunk.type, start, duration),
            i === 0 ? { decoderConfig: audio.decoderConfig } : undefined,
          )
        }
        sound.close()
      }
      await output.finalize()
      signal.throwIfAborted()
      if (!count) throw new Error('MEDIA_MUX_EMPTY')
      return target.file()
    },
    async cancel() {
      closed = true
      if (output.state !== 'finalized' && output.state !== 'canceled')
        await output.cancel().catch(() => undefined)
      await target.dispose()
    },
  }
}
