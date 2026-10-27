import { CLIP_LOCAL_PREVIEW } from '../config/local-preview'
import type { BrowserCompositionSnapshot } from '../model/browser-composition'
import type { BrowserSourceAccess } from '../model/browser-footage'
import type {
  BrowserPreviewFrameRequest,
  BrowserPreviewFrameResult,
  BrowserPreviewWorkerInput,
  BrowserPreviewWorkerOutput,
} from '../model/preview-worker-protocol'

/** Page-owned Worker RPC. Stale transferred bitmaps and late original access are released. */
export class BrowserPreviewWorker {
  private worker: Worker
  private nextId = 0
  private epoch = 0
  private fingerprint = ''
  private disposed = false
  private pending = new Map<
    number,
    {
      kind: 'initialize' | 'prepare' | 'frame'
      progress?: (done: number, total: number) => void
      resolve: (value: BrowserPreviewFrameResult | undefined) => void
      reject: (error: unknown) => void
      cleanup: () => void
    }
  >()
  private accesses = new Map<number, AbortController>()
  constructor(
    private readonly access: BrowserSourceAccess,
    createWorker = () =>
      new Worker(new URL('./preview.worker.ts', import.meta.url), { type: 'module' }),
  ) {
    this.worker = createWorker()
    this.worker.onmessage = (event: MessageEvent<BrowserPreviewWorkerOutput>) => {
      const message = event.data
      if (message.type === 'source') {
        if (this.disposed || message.snapshotFingerprint !== this.fingerprint) {
          this.send({ type: 'source', id: message.id, error: 'CLIP_SNAPSHOT_SUPERSEDED' })
          return
        }
        const controller = new AbortController()
        this.accesses.set(message.id, controller)
        void this.access(message.sourceId, message.fingerprint, controller.signal)
          .then(
            (value) => {
              if (
                !this.disposed &&
                !controller.signal.aborted &&
                message.snapshotFingerprint === this.fingerprint
              )
                this.send({ type: 'source', id: message.id, access: value })
            },
            (error: unknown) => {
              if (!this.disposed)
                this.send({
                  type: 'source',
                  id: message.id,
                  error: error instanceof Error ? error.message : 'CLIP_SOURCE_UNAVAILABLE',
                })
            },
          )
          .finally(() => this.accesses.delete(message.id))
        return
      }
      const waiter = this.pending.get(message.id)
      if (!waiter) {
        if (message.type === 'rendered') message.result.bitmap.close()
        return
      }
      if (message.type === 'preparing') {
        if (message.epoch === this.epoch && waiter.kind === 'prepare' && !this.disposed)
          waiter.progress?.(message.done, message.total)
        return
      }
      waiter.cleanup()
      this.pending.delete(message.id)
      if (message.type === 'failed') waiter.reject(new Error(message.error))
      else if (message.type === 'rendered') {
        if (
          message.epoch !== this.epoch ||
          message.result.fingerprint !== this.fingerprint ||
          this.disposed
        ) {
          message.result.bitmap.close()
          waiter.reject(new Error('CLIP_SNAPSHOT_SUPERSEDED'))
        } else waiter.resolve(message.result)
      } else if (message.type === 'prepared' && (message.epoch !== this.epoch || this.disposed))
        waiter.reject(new Error('CLIP_SNAPSHOT_SUPERSEDED'))
      else waiter.resolve(undefined)
    }
    this.worker.onerror = (event) =>
      this.fail(new Error(event.message || 'CLIP_PREVIEW_WORKER_FAILED'))
    this.worker.onmessageerror = () => this.fail(new Error('CLIP_PREVIEW_WORKER_FAILED'))
  }
  private send(message: BrowserPreviewWorkerInput) {
    if (!this.disposed) this.worker.postMessage(message)
  }
  private call(
    message: Extract<BrowserPreviewWorkerInput, { type: 'initialize' | 'prepare' | 'frame' }>,
    signal: AbortSignal,
    progress?: (done: number, total: number) => void,
  ) {
    return new Promise<BrowserPreviewFrameResult | undefined>((resolve, reject) => {
      signal.throwIfAborted()
      const cancelled = () => {
        this.pending.get(message.id)?.cleanup()
        this.pending.delete(message.id)
        reject(signal.reason)
      }
      const timer = setTimeout(
        () => {
          this.pending.get(message.id)?.cleanup()
          this.pending.delete(message.id)
          reject(new Error('CLIP_PREVIEW_TIMEOUT'))
          this.cancel()
        },
        message.type === 'prepare'
          ? CLIP_LOCAL_PREVIEW.preparationTimeoutMs
          : CLIP_LOCAL_PREVIEW.operationTimeoutMs,
      )
      const cleanup = () => {
        clearTimeout(timer)
        signal.removeEventListener('abort', cancelled)
      }
      signal.addEventListener('abort', cancelled, { once: true })
      this.pending.set(message.id, { kind: message.type, resolve, reject, cleanup, progress })
      this.send(message)
    })
  }
  async initialize(snapshot: BrowserCompositionSnapshot, signal: AbortSignal) {
    this.cancel()
    this.fingerprint = snapshot.snapshotFingerprint
    for (const controller of this.accesses.values()) controller.abort()
    this.accesses.clear()
    await this.call({ type: 'initialize', id: ++this.nextId, snapshot }, signal)
  }
  async render(request: BrowserPreviewFrameRequest, signal: AbortSignal) {
    return (await this.call(
      { type: 'frame', id: ++this.nextId, epoch: this.epoch, ...request },
      signal,
    ))!
  }
  async prepare(signal: AbortSignal, progress?: (done: number, total: number) => void) {
    await this.call({ type: 'prepare', id: ++this.nextId, epoch: this.epoch }, signal, progress)
  }
  cancel() {
    this.epoch++
    for (const [id, controller] of this.accesses) {
      controller.abort()
      this.send({ type: 'source', id, error: 'CLIP_SNAPSHOT_SUPERSEDED' })
    }
    this.accesses.clear()
    this.send({ type: 'cancel', epoch: this.epoch })
    for (const [id, waiter] of this.pending)
      if (waiter.kind === 'frame' || waiter.kind === 'prepare') {
        waiter.cleanup()
        waiter.reject(new Error('CLIP_SNAPSHOT_SUPERSEDED'))
        this.pending.delete(id)
      }
  }
  private fail(error: unknown) {
    for (const waiter of this.pending.values()) {
      waiter.cleanup()
      waiter.reject(error)
    }
    this.pending.clear()
  }
  dispose() {
    if (this.disposed) return
    this.cancel()
    this.send({ type: 'dispose' })
    this.disposed = true
    this.fail(new Error('CLIP_SNAPSHOT_SUPERSEDED'))
    for (const controller of this.accesses.values()) controller.abort()
    this.accesses.clear()
    this.worker.terminate()
  }
}
