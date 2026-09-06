/** What a container says about itself, read WITHOUT decoding a single frame.
 *
 *  A clip is uploaded exactly as the user picked it (VIDEO-4) — nothing here transcodes,
 *  downscales or extracts a poster — so this is the only thing the browser ever asks of the
 *  file, and it asks the platform's own demuxer through a `<video preload="metadata">`. */
export interface VideoMetadata {
  durationMs: number
  width: number
  height: number
}

/** How long to wait for `loadedmetadata` before calling the file unreadable. Generous for a
 *  200 MB clip off a slow phone filesystem, and bounded so a container the platform silently
 *  refuses to parse cannot leave a card stuck at 읽는 중 forever. */
const METADATA_TIMEOUT_MS = 15_000

export class VideoUnreadableError extends Error {
  constructor(message = 'the video metadata could not be read') {
    super(message)
    this.name = 'VideoUnreadableError'
  }
}

/** Reads a picked file's duration and dimensions.
 *
 *  The element is off-DOM and muted: it never renders, never plays, and never autoplays past
 *  the metadata it was asked for. The object URL is revoked on every path — resolve, reject
 *  and timeout alike — because a leaked one pins the whole file in memory.
 */
export function readVideoMetadata(file: Blob): Promise<VideoMetadata> {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file)
    const element = document.createElement('video')
    element.preload = 'metadata'
    element.muted = true

    let settled = false
    const done = (finish: () => void) => {
      if (settled) return
      settled = true
      clearTimeout(timer)
      element.removeAttribute('src')
      // `load()` after clearing the source is what actually stops a fetch already in
      // flight; without it a 200 MB read keeps going for a file nobody is waiting on.
      element.load()
      URL.revokeObjectURL(url)
      finish()
    }

    const timer = setTimeout(
      () => done(() => reject(new VideoUnreadableError())),
      METADATA_TIMEOUT_MS,
    )

    element.addEventListener('loadedmetadata', () => {
      const seconds = element.duration
      const width = element.videoWidth
      const height = element.videoHeight
      // A stream with no known duration reports Infinity or NaN, and one with no video track
      // reports 0×0. Neither can describe a clip this post could show, so both are unreadable
      // rather than a confirm the server would refuse.
      if (!Number.isFinite(seconds) || seconds <= 0 || width <= 0 || height <= 0) {
        done(() => reject(new VideoUnreadableError()))
        return
      }
      done(() => resolve({ durationMs: Math.round(seconds * 1000), width, height }))
    })
    element.addEventListener('error', () => done(() => reject(new VideoUnreadableError())))

    element.src = url
  })
}

/** `m:ss`, the badge on a video tile. Seconds are padded so the badge keeps one width and the
 *  strip does not reflow as clips of different lengths load. */
export function formatDuration(durationMs: number): string {
  const total = Math.max(0, Math.round(durationMs / 1000))
  const minutes = Math.floor(total / 60)
  const seconds = total % 60
  return `${minutes}:${String(seconds).padStart(2, '0')}`
}
