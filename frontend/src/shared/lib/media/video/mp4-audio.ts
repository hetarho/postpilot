import { ALL_FORMATS, BlobSource, Input } from 'mediabunny'

/** Inspect packet metadata before native decoding can allocate a whole original's PCM. */
export async function mp4AudioDecodedBytes(blob: Blob, sampleRate: number, signal: AbortSignal) {
  signal.throwIfAborted()
  const input = new Input({ formats: ALL_FORMATS, source: new BlobSource(blob) })
  try {
    const tracks = await input.getAudioTracks()
    let bytes = 0
    for (const track of tracks) {
      const duration = await track.computeDuration()
      signal.throwIfAborted()
      if (
        !Number.isFinite(duration) ||
        duration <= 0 ||
        !Number.isSafeInteger(track.numberOfChannels) ||
        track.numberOfChannels <= 0
      )
        throw new Error('Invalid original audio metadata')
      // Include codec padding and the stereo rematrix buffer when needed.
      const channels = track.numberOfChannels === 2 ? 2 : track.numberOfChannels + 2
      bytes += Math.ceil((duration + 0.1) * sampleRate) * channels * 4
    }
    if (!Number.isSafeInteger(bytes)) throw new Error('Invalid original audio metadata')
    return bytes
  } finally {
    input.dispose()
  }
}

/** Read only ISO BMFF/QuickTime box headers; never mistake decoder failure for a silent file. */
export async function mp4HasAudio(blob: Blob, signal: AbortSignal) {
  let boxes = 0
  async function inspect(start: number, end: number, depth: number): Promise<boolean> {
    for (let offset = start; offset < end;) {
      signal.throwIfAborted()
      if (++boxes > 10000 || offset + 8 > end) throw new Error('Invalid MP4 boxes')
      const header = new DataView(
        await blob.slice(offset, Math.min(offset + 24, end)).arrayBuffer(),
      )
      const tag = String.fromCharCode(...[4, 5, 6, 7].map((i) => header.getUint8(i)))
      const small = header.getUint32(0)
      const size =
        small === 0
          ? end - offset
          : small === 1 && header.byteLength >= 16
            ? Number(header.getBigUint64(8))
            : small
      const headerBytes = small === 1 ? 16 : 8
      if (!Number.isSafeInteger(size) || size < headerBytes || offset + size > end)
        throw new Error('Invalid MP4 box size')
      if (depth === 3 && tag === 'hdlr') {
        if (size < headerBytes + 12) throw new Error('Invalid MP4 handler')
        const data = new Uint8Array(
          await blob.slice(offset + headerBytes + 8, offset + headerBytes + 12).arrayBuffer(),
        )
        if (data.length !== 4) throw new Error('Invalid MP4 handler')
        if (String.fromCharCode(...data) === 'soun') return true
      }
      if (
        ['moov', 'trak', 'mdia'][depth] === tag &&
        (await inspect(offset + headerBytes, offset + size, depth + 1))
      )
        return true
      offset += size
    }
    return false
  }
  return inspect(0, blob.size, 0)
}
