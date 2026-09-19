import { CLIP_BROWSER_RENDER } from '@/entities/clip-design'
import { BrowserOriginals } from './originals'

interface SourceFramePorts {
  read: (fingerprint: string) => Promise<Blob>
  dispose?: () => void
  open: (
    blob: Blob,
    signal: AbortSignal,
  ) => Promise<{
    frame: (timeMs: number, signal: AbortSignal) => Promise<ImageBitmap>
    close: () => void
  }>
}

/** One source read per render, reused by every cut citing that fingerprint. */
export class BrowserSourceFrames {
  private sources = new Map<string, ReturnType<SourceFramePorts['open']>>()
  constructor(
    private ports: SourceFramePorts,
    private signal: AbortSignal,
  ) {}
  async frame(fingerprint: string, timeMs: number) {
    this.signal.throwIfAborted()
    let source = this.sources.get(fingerprint)
    if (!source) {
      source = this.ports.read(fingerprint).then(async (blob) => {
        this.signal.throwIfAborted()
        return this.ports.open(blob, this.signal)
      })
      this.sources.set(fingerprint, source)
    }
    const decoded = await source
    this.signal.throwIfAborted()
    return decoded.frame(timeMs, this.signal)
  }
  dispose() {
    for (const source of this.sources.values())
      void source.then(
        (s) => s.close(),
        () => {},
      )
    this.sources.clear()
    this.ports.dispose?.()
  }
}

function mediaReady(video: HTMLVideoElement, event: 'loadeddata' | 'seeked', signal: AbortSignal) {
  return new Promise<void>((resolve, reject) => {
    const finish = (error?: unknown) => {
      clearTimeout(timer)
      video.removeEventListener(event, ready)
      video.removeEventListener('error', failed)
      signal.removeEventListener('abort', aborted)
      if (error) reject(error)
      else resolve()
    }
    const ready = () => finish()
    const failed = () => finish(new Error('CLIP_SOURCE_UNAVAILABLE'))
    const aborted = () => finish(signal.reason)
    const timer = setTimeout(failed, CLIP_BROWSER_RENDER.sourceTimeoutMs)
    video.addEventListener(event, ready, { once: true })
    video.addEventListener('error', failed, { once: true })
    signal.addEventListener('abort', aborted, { once: true })
    if (signal.aborted) aborted()
  })
}

/** HTML video owns container parsing on the window; only snapshots cross to the worker. */
export async function openBrowserSourceVideo(blob: Blob, signal: AbortSignal) {
  signal.throwIfAborted()
  const video = document.createElement('video')
  const url = URL.createObjectURL(blob)
  video.muted = true
  video.playsInline = true
  video.preload = 'auto'
  const close = () => {
    video.pause()
    video.removeAttribute('src')
    video.load()
    URL.revokeObjectURL(url)
  }
  try {
    const loaded = mediaReady(video, 'loadeddata', signal)
    video.src = url
    await loaded
    return {
      async frame(timeMs: number, signal: AbortSignal) {
        signal.throwIfAborted()
        const time = timeMs / 1000
        if (video.currentTime !== time) {
          const sought = mediaReady(video, 'seeked', signal)
          video.currentTime = time
          await sought
        }
        signal.throwIfAborted()
        return createImageBitmap(video)
      },
      close,
    }
  } catch (error) {
    close()
    throw error
  }
}

export function createBrowserSourceFrames(
  localSources: readonly { fingerprint: string; url: string }[],
  resolvePlayback: (fingerprint: string) => Promise<string>,
  signal: AbortSignal,
  originals?: BrowserOriginals,
) {
  const reader = originals ?? new BrowserOriginals(localSources, resolvePlayback, signal)
  return new BrowserSourceFrames(
    {
      read: reader.get,
      dispose: originals ? undefined : () => reader.dispose(),
      open: openBrowserSourceVideo,
    },
    signal,
  )
}
