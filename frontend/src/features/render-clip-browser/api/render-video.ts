import { createBrowserSourceFrames } from '../lib/source-frames'
import { BrowserOriginals } from '../lib/originals'
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
  CLIP_VIDEO_DECODING,
} from '@/entities/clip-preview'

/** Heavy composition/encoding lives in the worker; DOM video supplies transferable snapshots. */
export function renderBrowserVideo(
  input: BrowserVideoInput,
  localSources: readonly { fingerprint: string; url: string; file?: File }[],
  resolvePlayback: (fingerprint: string) => Promise<string>,
  signal?: AbortSignal,
  originals?: BrowserOriginals,
  captionFrames?: CaptionFrameLoader,
): BrowserVideoRender {
  const controller = new AbortController()
  const sources = input.snapshot
    ? undefined
    : createBrowserSourceFrames(localSources, resolvePlayback, controller.signal, originals)
  const localOriginals =
    originals ?? new BrowserOriginals(localSources, resolvePlayback, controller.signal)
  const worker = createClipVideoWorker()
  // The sheets a sequence caption is drawn from stay on THIS side: the worker asks
  // for the frame it is drawing, and the page holds one run per caption (CLIP-159).
  const sheets = captionFrames ? new CaptionSheets(captionFrames) : undefined
  const send = (message: VideoWorkerInput, transfer: Transferable[] = []) =>
    worker.postMessage(message, transfer)
  let stopped = false
  let cleanupTimer: ReturnType<typeof setTimeout> | undefined
  let latest: BrowserVideoProgress | undefined
  let wake: (() => void) | undefined
  let resolve!: (track: BrowserVideoTrack) => void
  let reject!: (error: unknown) => void
  const result = new Promise<BrowserVideoTrack>((yes, no) => {
    resolve = yes
    reject = no
  })
  const terminate = () => {
    clearTimeout(cleanupTimer)
    worker.terminate()
  }
  const stop = (completed = false) => {
    stopped = true
    controller.abort()
    if (input.snapshot && !completed) {
      try {
        send({ type: 'cancel' })
      } catch {
        terminate()
      }
      cleanupTimer = setTimeout(terminate, CLIP_VIDEO_DECODING.workerCleanupMs)
    } else terminate()
    sources?.dispose()
    if (!originals) localOriginals.dispose()
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
    const message = event.data
    if (stopped) {
      if (message.type === 'cancelled') terminate()
      return
    }
    if (message.type === 'sourceAccess') {
      void localOriginals
        .source(message.fingerprint)
        .then(
          (access) => {
            if (!stopped) send({ type: 'sourceAccess', requestId: message.requestId, access })
          },
          (error: unknown) => {
            if (!stopped)
              send({
                type: 'sourceAccess',
                requestId: message.requestId,
                error: error instanceof Error ? error.message : 'CLIP_SOURCE_UNAVAILABLE',
              })
          },
        )
        .catch(fail)
    } else if (message.type === 'source') {
      if (!sources) {
        fail(new Error('CLIP_SOURCE_DOM_PATH_REFUSED'))
        return
      }
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
    } else if (message.type === 'error')
      fail(Object.assign(new Error(message.error), { measurements: message.measurements }))
    else if (message.type === 'done') {
      stop(true)
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
