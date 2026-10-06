import type { ClipAnalysisOriginalMeasurement } from '@/entities/clip-project'
import { ANALYSIS_PREPARATION_LIMITS as limits } from '../config/limits'
import type {
  AnalysisCopyArtifact,
  AnalysisCopySlot,
  AnalysisEncoder,
  AnalysisSource,
} from '../model/types'

export function createAnalysisEncoder(
  signal: AbortSignal,
  progress: (fraction: number) => void = () => {},
): AnalysisEncoder {
  signal.throwIfAborted()
  if (typeof Worker === 'undefined') throw new Error('CLIP_ANALYSIS_ENCODER_UNSUPPORTED')
  const worker = new Worker(new URL('./analysis.worker.ts', import.meta.url), { type: 'module' })
  let id = 0,
    closed = false
  let pending:
    | {
        id: number
        resolve: (value: unknown) => void
        reject: (error: unknown) => void
        timer: ReturnType<typeof setTimeout>
      }
    | undefined
  let cleanup: ReturnType<typeof setTimeout> | undefined
  const terminate = () => {
    if (cleanup) clearTimeout(cleanup)
    worker.terminate()
  }
  function close(reason: unknown = new DOMException('Preparation cancelled', 'AbortError')) {
    if (closed) return
    closed = true
    signal.removeEventListener('abort', abort)
    if (pending) {
      clearTimeout(pending.timer)
      pending.reject(reason)
      pending = undefined
    }
    cleanup = setTimeout(terminate, limits.cleanupTimeoutMs)
    try {
      worker.postMessage({ kind: 'cancel' })
    } catch {
      terminate()
    }
  }
  const abort = () => close(signal.reason)
  signal.addEventListener('abort', abort, { once: true })
  worker.onmessage = (event: MessageEvent) => {
    const message = event.data
    if (message.kind === 'cancelled') {
      terminate()
      return
    }
    if (closed || !pending || pending.id !== message.id) return
    if (message.kind === 'progress') {
      progress(message.fraction)
      return
    }
    const request = pending
    clearTimeout(request.timer)
    pending = undefined
    if (message.kind === 'error') request.reject(new Error(message.error))
    else request.resolve(message.result)
  }
  worker.onerror = (event) => {
    event.preventDefault()
    close(new Error(event.message || 'CLIP_ANALYSIS_WORKER_FAILED'))
  }
  worker.onmessageerror = () => close(new Error('CLIP_ANALYSIS_WORKER_FAILED'))
  async function request(
    kind: 'measure' | 'encode',
    source: AnalysisSource,
    slot?: AnalysisCopySlot,
  ) {
    signal.throwIfAborted()
    if (closed || pending) throw new Error('CLIP_ANALYSIS_QUEUE_LIMIT')
    return new Promise<unknown>((resolve, reject) => {
      const requestId = ++id
      const timer = setTimeout(
        () => close(new Error('CLIP_ANALYSIS_TIMEOUT')),
        limits.operationTimeoutMs,
      )
      pending = { id: requestId, resolve, reject, timer }
      try {
        worker.postMessage({ kind, id: requestId, source, slot })
      } catch (error) {
        close(error)
      }
    })
  }
  return {
    measure: (source) => request('measure', source) as Promise<ClipAnalysisOriginalMeasurement>,
    encode: (source, slot) => request('encode', source, slot) as Promise<AnalysisCopyArtifact>,
    close,
  }
}
