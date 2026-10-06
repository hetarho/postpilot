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

/** Compatibility for diagnostic callers that already collected packets.
 * Product export uses createMp4PacketMux directly and keeps no video array. */
export async function muxMp4(
  video: VideoTrack,
  audio: EncodedAudioTrack | undefined,
  signal: AbortSignal,
): Promise<Blob> {
  signal.throwIfAborted()
  const { createBoundedMediaOutput } = await import('./bounded-output')
  const { createMp4PacketMux } = await import('./stream-mp4')
  const packetBytes =
    video.chunks.reduce((n, packet) => n + packet.data.byteLength, 0) +
    (audio?.chunks.reduce((n, packet) => n + packet.data.byteLength, 0) ?? 0)
  const metadataBytes = (video.chunks.length + (audio?.chunks.length ?? 0)) * 64 + 8192
  const maxBytes = packetBytes + metadataBytes
  const target = await createBoundedMediaOutput(
    { maxBytes, pageBytes: Math.min(maxBytes, 256 * 1024) },
    signal,
  )
  const mux = await createMp4PacketMux(target, video.config.framerate ?? 0, audio, signal)
  try {
    for (const [i, packet] of video.chunks.entries())
      await mux.video(packet, i === 0 ? video.decoderConfig : undefined)
    return await mux.finalize()
  } finally {
    await mux.cancel()
  }
}
