import type { AudioRangeLimits, AudioSourceRange, SelectedAudioRange } from './range-audio'
import type { BrowserMediaSourceAccess } from '../video/range-source'

/** One owned range at a time; cancellation gives the decoder a bounded cleanup
 * turn before terminating the worker, and never publishes late PCM. */
export function createAudioRangeReader(signal: AbortSignal, limits: AudioRangeLimits) {
  signal.throwIfAborted()
  const worker = new Worker(new URL('./range-audio.worker.ts', import.meta.url), { type: 'module' })
  let nextId = 0,
    closed = false
  let pending:
    | {
        id: number
        resolve: (range: SelectedAudioRange | undefined) => void
        reject: (error: unknown) => void
        timer: ReturnType<typeof setTimeout>
      }
    | undefined
  let cleanupTimer: ReturnType<typeof setTimeout> | undefined
  const terminate = () => {
    if (cleanupTimer) clearTimeout(cleanupTimer)
    worker.terminate()
  }
  const close = (reason: unknown = new DOMException('Audio range cancelled', 'AbortError')) => {
    if (closed) return
    closed = true
    signal.removeEventListener('abort', abort)
    if (pending) {
      clearTimeout(pending.timer)
      pending.reject(reason)
      pending = undefined
    }
    cleanupTimer = setTimeout(terminate, limits.cleanupTimeoutMs)
    try {
      worker.postMessage({ kind: 'cancel' })
    } catch {
      terminate()
    }
  }
  const abort = () => close(signal.reason)
  signal.addEventListener('abort', abort, { once: true })
  worker.onmessage = (
    event: MessageEvent<
      | { kind: 'cancelled' }
      | { kind: 'result'; id: number; result: SelectedAudioRange | undefined }
      | { kind: 'error'; id: number; error: string }
    >,
  ) => {
    const message = event.data
    if (message.kind === 'cancelled') {
      terminate()
      return
    }
    if (closed || !pending || pending.id !== message.id) return
    const request = pending
    clearTimeout(request.timer)
    pending = undefined
    if (message.kind === 'error') request.reject(new Error(message.error))
    else request.resolve(message.result)
  }
  worker.onerror = (event) => {
    event.preventDefault()
    close(new Error(event.message || 'CLIP_SOURCE_AUDIO_WORKER_FAILED'))
  }
  worker.onmessageerror = () => close(new Error('CLIP_SOURCE_AUDIO_WORKER_FAILED'))
  return {
    close,
    decode(
      access: BrowserMediaSourceAccess,
      range: AudioSourceRange,
      availablePcmBytes = limits.maxPcmBytes,
    ) {
      signal.throwIfAborted()
      if (closed) return Promise.reject(new Error('CLIP_SOURCE_AUDIO_WORKER_CLOSED'))
      if (pending) return Promise.reject(new Error('CLIP_SOURCE_AUDIO_QUEUE_LIMIT'))
      return new Promise<SelectedAudioRange | undefined>((resolve, reject) => {
        const id = ++nextId
        const timer = setTimeout(
          () => close(new Error('CLIP_SOURCE_AUDIO_TIMEOUT')),
          limits.operationTimeoutMs,
        )
        pending = { id, resolve, reject, timer }
        try {
          worker.postMessage({
            kind: 'decode',
            id,
            access,
            range,
            limits: { ...limits, maxPcmBytes: availablePcmBytes },
          })
        } catch (error) {
          close(error)
        }
      })
    },
  }
}
