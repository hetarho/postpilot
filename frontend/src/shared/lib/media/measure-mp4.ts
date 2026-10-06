export interface Mp4TrackMeasurements {
  width: number
  height: number
  codec: string
  videoFrames: number
  videoDurationUs: number
  presentationValid: boolean
  hasAudio: boolean
  audioCodec: string
  audioRate: number
}
/** Reads the finalized file's own track metadata and every packet timestamp.
 * Metadata-only packet reads retain neither encoded bytes nor decoded frames. */
export async function measureMp4Output(
  file: Blob,
  frameRate: number,
  maxFrames: number,
  signal: AbortSignal,
): Promise<Mp4TrackMeasurements> {
  const { Input, BlobSource, MP4, EncodedPacketSink } = await import('mediabunny')
  signal.throwIfAborted()
  const input = new Input({ source: new BlobSource(file), formats: [MP4] })
  const abort = () => input.dispose()
  signal.addEventListener('abort', abort, { once: true })
  try {
    const videoTracks = await input.getVideoTracks(),
      audioTracks = await input.getAudioTracks()
    if (videoTracks.length !== 1 || audioTracks.length > 1)
      throw new Error('MEDIA_OUTPUT_TRACKS_INVALID')
    const video = videoTracks[0]!
    const config = await video.getDecoderConfig()
    const audio = audioTracks[0]
    const sound = await audio?.getDecoderConfig()
    const frames = new Set<number>()
    let count = 0,
      end = 0,
      valid = true
    for await (const packet of new EncodedPacketSink(video).packets(undefined, undefined, {
      metadataOnly: true,
    })) {
      signal.throwIfAborted()
      if (++count > maxFrames) throw new Error('MEDIA_OUTPUT_FRAME_LIMIT')
      const frame = Math.round(packet.timestamp * frameRate)
      valid &&=
        frame >= 0 &&
        frame < maxFrames &&
        !frames.has(frame) &&
        Math.abs(packet.timestamp - frame / frameRate) <= 1e-6 &&
        Math.abs(packet.duration - 1 / frameRate) <= 1e-6
      frames.add(frame)
      end = Math.max(end, packet.timestamp + packet.duration)
    }
    signal.throwIfAborted()
    return {
      width: await video.getDisplayWidth(),
      height: await video.getDisplayHeight(),
      codec: config?.codec ?? '',
      videoFrames: count,
      videoDurationUs: Math.round(end * 1e6),
      presentationValid: valid && frames.size === count && frames.has(0),
      hasAudio: !!audio,
      audioCodec: sound?.codec ?? '',
      audioRate: sound?.sampleRate ?? 0,
    }
  } finally {
    signal.removeEventListener('abort', abort)
    input.dispose()
  }
}
