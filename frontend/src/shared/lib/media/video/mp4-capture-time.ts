/** The installed demuxer skips mvhd's creation fields; descriptive date tags are not substitutes. */
const MAC_TO_UNIX_SECONDS = 2_082_844_800n
const FIRST_CAPTURE_SECONDS = BigInt(Date.UTC(2000, 0, 1) / 1000)
const LAST_CAPTURE_SECONDS = BigInt(Date.UTC(9999, 11, 31, 23, 59, 59) / 1000)
const MAX_HEADER_BYTES = 256 * 1024

export async function readMp4CaptureTime(
  blob: Blob,
  signal?: AbortSignal,
): Promise<string | undefined> {
  let readBytes = 0
  async function inspect(start: number, end: number, movie: boolean): Promise<string | undefined> {
    for (let at = start; at < end;) {
      signal?.throwIfAborted()
      if (at + 8 > end || readBytes + 32 > MAX_HEADER_BYTES) return
      const view = new DataView(await blob.slice(at, Math.min(at + 32, end)).arrayBuffer())
      signal?.throwIfAborted()
      readBytes += view.byteLength
      if (view.byteLength < 8) return
      const tag = String.fromCharCode(...[4, 5, 6, 7].map((i) => view.getUint8(i)))
      const small = view.getUint32(0)
      const size =
        small === 0
          ? end - at
          : small === 1 && view.byteLength >= 16
            ? Number(view.getBigUint64(8))
            : small
      const header = small === 1 ? 16 : 8
      if (!Number.isSafeInteger(size) || size < header || at + size > end) return
      if (movie && tag === 'mvhd') {
        if (size < header + 8 || view.byteLength < header + 8) return
        const version = view.getUint8(header)
        if (version !== 0 && version !== 1) return
        if (version === 1 && (size < header + 12 || view.byteLength < header + 12)) return
        const seconds =
          (version === 1 ? view.getBigUint64(header + 4) : BigInt(view.getUint32(header + 4))) -
          MAC_TO_UNIX_SECONDS
        if (seconds < FIRST_CAPTURE_SECONDS || seconds > LAST_CAPTURE_SECONDS) return
        return new Date(Number(seconds) * 1000).toISOString()
      }
      if (!movie && tag === 'moov') return inspect(at + header, at + size, true)
      at += size
    }
  }
  try {
    return await inspect(0, blob.size, false)
  } catch {
    signal?.throwIfAborted()
    // Unknown/truncated metadata is absent, never File.lastModified or a guessed date.
    return undefined
  }
}
