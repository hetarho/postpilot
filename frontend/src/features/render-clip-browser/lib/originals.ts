/** A render's video and audio paths share one in-memory read of each original. */
export class BrowserOriginals {
  private blobs = new Map<string, Promise<Blob>>()
  constructor(
    private local: readonly { fingerprint: string; url: string }[],
    private resolvePlayback: (fingerprint: string) => Promise<string>,
    private signal: AbortSignal,
    private load: (url: string, signal: AbortSignal) => Promise<Blob> = async (url, signal) => {
      const response = await fetch(url, { signal })
      if (!response.ok) throw new Error('CLIP_SOURCE_UNAVAILABLE')
      return response.blob()
    },
  ) {}
  get = (fingerprint: string) => {
    this.signal.throwIfAborted()
    let blob = this.blobs.get(fingerprint)
    if (!blob) {
      blob = (async () => {
        // Only a file this page holds is read from its object URL. A server-held
        // original's URL in that list is whatever the page last showed: empty until
        // a scene asked for it, or a presigned read that may have expired. Fetching
        // the empty one reads the app's own page, which no demuxer opens.
        const local = this.local.find((source) => source.fingerprint === fingerprint)?.url
        const url = local?.startsWith('blob:') ? local : await this.resolvePlayback(fingerprint)
        this.signal.throwIfAborted()
        return this.load(url, this.signal)
      })()
      this.blobs.set(fingerprint, blob)
    }
    return blob
  }
  dispose() {
    this.blobs.clear()
  }
}
