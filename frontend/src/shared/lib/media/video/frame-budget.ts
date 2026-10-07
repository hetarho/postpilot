import { MediaRangeError } from './range-source'

/** Owned presentation frames, separate from conservative decoder reservations. */
export class VideoFrameBudget {
  private frames = 0
  private bytes = 0
  private peakFrames = 0
  private peakBytes = 0
  private waiters: {
    bytes: number
    signal: AbortSignal
    grant: (release: () => void) => void
    reject: (error: unknown) => void
    abort: () => void
  }[] = []
  constructor(
    private maxFrames: number,
    private maxBytes: number,
  ) {
    if (
      !Number.isSafeInteger(maxFrames) ||
      maxFrames <= 0 ||
      !Number.isSafeInteger(maxBytes) ||
      maxBytes <= 0
    )
      throw new MediaRangeError('CLIP_SOURCE_MEMORY_LIMIT')
  }
  acquire(bytes: number, signal: AbortSignal): Promise<() => void> {
    signal.throwIfAborted()
    if (!Number.isSafeInteger(bytes) || bytes <= 0 || bytes > this.maxBytes)
      return Promise.reject(new MediaRangeError('CLIP_SOURCE_MEMORY_LIMIT'))
    if (this.waiters.length >= this.maxFrames)
      return Promise.reject(new MediaRangeError('CLIP_SOURCE_QUEUE_LIMIT'))
    return new Promise((grant, reject) => {
      const entry = {
        bytes,
        signal,
        grant,
        reject,
        abort: () => {
          this.waiters = this.waiters.filter((w) => w !== entry)
          reject(signal.reason)
          this.drain()
        },
      }
      signal.addEventListener('abort', entry.abort, { once: true })
      this.waiters.push(entry)
      this.drain()
    })
  }
  private drain() {
    while (
      this.waiters.length &&
      this.frames < this.maxFrames &&
      this.bytes + this.waiters[0]!.bytes <= this.maxBytes
    ) {
      const entry = this.waiters.shift()!
      entry.signal.removeEventListener('abort', entry.abort)
      if (entry.signal.aborted) {
        entry.reject(entry.signal.reason)
        continue
      }
      this.frames++
      this.bytes += entry.bytes
      this.peakFrames = Math.max(this.peakFrames, this.frames)
      this.peakBytes = Math.max(this.peakBytes, this.bytes)
      let closed = false
      entry.grant(() => {
        if (closed) return
        closed = true
        this.frames--
        this.bytes -= entry.bytes
        this.drain()
      })
    }
  }
  snapshot() {
    return {
      liveFrames: this.frames,
      liveBytes: this.bytes,
      peakFrames: this.peakFrames,
      peakBytes: this.peakBytes,
      queued: this.waiters.length,
    }
  }
}
