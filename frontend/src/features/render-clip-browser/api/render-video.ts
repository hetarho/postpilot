import { createBrowserSourceFrames } from '../lib/source-frames'
import type { BrowserOriginals } from '../lib/originals'
import {
  CaptionSheets,
  createClipVideoWorker,
  type CaptionFrameLoader,
  type BrowserVideoInput,
  type BrowserVideoProgress,
  type BrowserVideoRender,
  type BrowserVideoTrack,
  type VideoWorkerInput,
  type VideoWorkerOutput,
} from '@/entities/clip-preview'

/** Heavy composition/encoding lives in the worker; DOM video supplies transferable snapshots. */
export function renderBrowserVideo(
  input: BrowserVideoInput,
  localSources: readonly { fingerprint: string; url: string }[],
  resolvePlayback: (fingerprint: string) => Promise<string>,
  signal?: AbortSignal,
  originals?: BrowserOriginals,
  captionFrames?: CaptionFrameLoader,
): BrowserVideoRender {
  const controller = new AbortController()
  const sources = createBrowserSourceFrames(
    localSources,
    resolvePlayback,
    controller.signal,
    originals,
  )
  const worker = createClipVideoWorker()
  // The sheets a sequence caption is drawn from stay on THIS side: the worker asks
  // for the frame it is drawing, and the page holds one run per caption (CLIP-159).
  const sheets = captionFrames ? new CaptionSheets(captionFrames) : undefined
  const send = (message: VideoWorkerInput, transfer: Transferable[] = []) =>
    worker.postMessage(message, transfer)
  let stopped = false
  let latest: BrowserVideoProgress | undefined
  let wake: (() => void) | undefined
  let resolve!: (track: BrowserVideoTrack) => void
  let reject!: (error: unknown) => void
  const result = new Promise<BrowserVideoTrack>((yes, no) => {
    resolve = yes
    reject = no
  })
  const stop = () => {
    stopped = true
    controller.abort()
    worker.terminate()
    sources.dispose()
    sheets?.dispose()
    signal?.removeEventListener('abort', cancel)
    wake?.()
  }
  const fail = (error: unknown) => {
    if (!stopped) {
      stop()
      reject(error)
    }
  }
  const cancel = () => fail(new DOMException('Render cancelled', 'AbortError'))
  worker.onmessage = (event: MessageEvent<VideoWorkerOutput>) => {
    if (stopped) return
    const message = event.data
    if (message.type === 'source') {
      void sources.frame(message.fingerprint, message.timeMs).then((bitmap) => {
        if (stopped) bitmap.close()
        else {
          try {
            send({ type: 'source', requestId: message.requestId, bitmap }, [bitmap])
          } catch (error) {
            bitmap.close()
            fail(error)
          }
        }
      }, fail)
    } else if (message.type === 'frames') {
      const answer = sheets
        ? sheets.cell(message.instanceId, message.frame, controller.signal)
        : Promise.reject(new Error('CLIP_CAPTION_FRAMES_UNAVAILABLE'))
      void answer.then((cell) => {
        if (stopped) {
          cell?.bitmap.close()
          return
        }
        try {
          send(
            {
              type: 'frames',
              requestId: message.requestId,
              bitmap: cell?.bitmap,
              x: cell?.x ?? 0,
              y: cell?.y ?? 0,
              width: cell?.width ?? 0,
              height: cell?.height ?? 0,
            },
            cell ? [cell.bitmap] : [],
          )
        } catch (error) {
          cell?.bitmap.close()
          fail(error)
        }
      }, fail)
    } else if (message.type === 'progress') {
      latest = message.progress
      wake?.()
    } else if (message.type === 'error') fail(new Error(message.error))
    else {
      stop()
      resolve(message.track)
    }
  }
  worker.onerror = (event) => {
    event.preventDefault()
    fail(new Error(event.message || 'CLIP_VIDEO_WORKER_FAILED'))
  }
  worker.onmessageerror = () => fail(new Error('CLIP_VIDEO_WORKER_FAILED'))
  signal?.addEventListener('abort', cancel, { once: true })
  if (signal?.aborted) cancel()
  else {
    try {
      send({ type: 'start', input })
    } catch (error) {
      fail(error)
    }
  }
  return {
    result,
    cancel,
    progress: {
      async *[Symbol.asyncIterator]() {
        while (!stopped || latest) {
          if (latest) {
            const value = latest
            latest = undefined
            yield value
          } else
            await new Promise<void>((resolve) => {
              wake = resolve
            })
        }
      },
    },
  }
}
