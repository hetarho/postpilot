import type { ClipSourceBatch, ClipSourceMetadata, ReadyClipBatch } from '@/entities/clip-project'

export interface SourcePipeline {
  read(files: readonly File[], signal: AbortSignal): Promise<ClipSourceMetadata[]>
  reserve(
    projectId: string,
    manifest: ClipSourceMetadata[],
    signal?: AbortSignal,
  ): Promise<{
    batch: ClipSourceBatch
    uploads: Array<{ sourceId: string; putUrl: string; headers: Record<string, string> }>
  }>
  put(
    url: string,
    headers: Record<string, string>,
    file: File,
    progress: (percent: number) => void,
    signal: AbortSignal,
  ): Promise<void>
  confirm(batchId: string, sourceId: string, signal?: AbortSignal): Promise<ClipSourceBatch>
  discard(batchId: string): Promise<void>
  createURL(file: File): string
  revokeURL(url: string): void
}
export interface LocalClipSource {
  file: File
  previewURL: string
  metadata: ClipSourceMetadata
  percent: number
  confirmed: boolean
}
export interface ClipUploadState {
  phase:
    | 'idle'
    | 'reading'
    | 'uploading'
    | 'ready'
    | 'accepting'
    | 'owned'
    | 'finished'
    | 'cancelling'
    | 'failed'
  entries: readonly LocalClipSource[]
  summaries?: readonly { filename: string; status: 'done' | 'failed' }[]
  attempt?: { batchId: string; jobId?: string }
  readyBatch?: ReadyClipBatch
  error?: unknown
}

const activeSessions = new Set<ClipSourceSession>()
/** Logout/expired authentication releases runtime media immediately, before navigation. */
export function discardClipSourceSessions() {
  for (const session of activeSessions) session.dispose()
}

/** A page-owned session, never a singleton or a query-cache value. */
export class ClipSourceSession {
  private active = false
  private epoch = 0
  private controller?: AbortController
  private batchId?: string
  private requiredKey?: string
  private attempt?: { batch: ReadyClipBatch; epoch: number; jobId?: string }
  private entries = new Map<string, LocalClipSource>()
  private listeners = new Set<() => void>()
  private state: ClipUploadState = { phase: 'idle', entries: [] }
  constructor(
    private projectId: string,
    private pipeline: SourcePipeline,
  ) {}
  getSnapshot = () => this.state
  subscribe = (listener: () => void) => {
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }
  activate() {
    this.active = true
    activeSessions.add(this)
  }
  changeConstraints(key: string) {
    const changed = this.requiredKey !== undefined && this.requiredKey !== key
    this.requiredKey = key
    if (
      !changed ||
      !this.active ||
      this.attempt ||
      (this.entries.size === 0 && this.state.phase !== 'reading')
    )
      return
    this.dispose()
    this.activate()
  }
  private publish(
    phase: ClipUploadState['phase'],
    extras: Pick<ClipUploadState, 'readyBatch' | 'error' | 'summaries'> = {},
  ) {
    this.state = {
      phase,
      entries: [...this.entries.values()],
      ...extras,
      ...(this.attempt
        ? { attempt: { batchId: this.attempt.batch.id, jobId: this.attempt.jobId } }
        : {}),
    }
    if (this.active) for (const listener of this.listeners) listener()
  }
  private clearFiles() {
    for (const entry of this.entries.values()) this.pipeline.revokeURL(entry.previewURL)
    this.entries.clear()
  }
  private current(token: number) {
    return this.active && token === this.epoch
  }
  dispose() {
    this.active = false
    activeSessions.delete(this)
    this.epoch++
    this.controller?.abort()
    this.controller = undefined
    this.clearFiles()
    this.batchId = undefined
    this.attempt = undefined
    this.publish('idle')
    for (const listener of this.listeners) listener()
    // No asynchronous discard from unmount/unload. The server's lease and orphan sweep own it.
  }
  /** Freeze selection before sending a start: a lost response must not expose cancel. */
  beginAttempt(batchId: string) {
    const batch = this.state.readyBatch
    if (!this.active || this.attempt || !batch || batch.id !== batchId) return false
    this.controller = undefined
    this.attempt = { batch, epoch: ++this.epoch }
    this.publish('accepting')
    return true
  }
  markOwned(batchId: string, jobId: string) {
    if (
      !this.active ||
      !jobId ||
      this.attempt?.batch.id !== batchId ||
      this.attempt.epoch !== this.epoch ||
      (this.attempt.jobId && this.attempt.jobId !== jobId)
    )
      return
    this.attempt.jobId = jobId
    this.batchId = undefined
    this.publish('owned')
  }
  /** Only a definite pre-acceptance refusal makes a batch selectable again. */
  rejectAttempt(batchId: string) {
    if (
      !this.active ||
      this.attempt?.batch.id !== batchId ||
      this.attempt.jobId ||
      this.attempt.epoch !== this.epoch
    )
      return
    const batch = this.attempt.batch
    this.attempt = undefined
    this.publish('ready', { readyBatch: batch })
  }
  finishAttempt(jobId: string, status: 'done' | 'failed') {
    if (!this.active || this.attempt?.jobId !== jobId || this.attempt.epoch !== this.epoch) return
    const summaries = [...this.entries.values()].map((e) => ({
      filename: e.metadata.filename,
      status,
    }))
    this.epoch++
    this.controller?.abort()
    this.controller = undefined
    this.clearFiles()
    this.batchId = undefined
    this.attempt = undefined
    this.publish('finished', { summaries })
  }
  async cancel() {
    if (!this.active || this.attempt) return
    const token = ++this.epoch
    this.controller?.abort()
    this.clearFiles()
    const batchId = this.batchId
    this.publish(batchId ? 'cancelling' : 'idle')
    if (!batchId) return
    try {
      await this.pipeline.discard(batchId)
      if (this.current(token)) {
        this.batchId = undefined
        this.publish('idle')
      }
    } catch (error) {
      if (this.current(token)) this.publish('failed', { error })
    }
  }
  async select(files: readonly File[], validate?: (manifest: ClipSourceMetadata[]) => void) {
    if (!files.length || !this.active || this.attempt) return
    const token = ++this.epoch
    this.controller?.abort()
    const controller = new AbortController()
    this.controller = controller
    const previous = this.batchId
    this.clearFiles()
    this.publish('reading')
    try {
      if (previous) {
        await this.pipeline.discard(previous)
        if (!this.current(token)) return
        this.batchId = undefined
      }
      const manifest = await this.pipeline.read(files, controller.signal)
      if (!this.current(token)) return
      validate?.(manifest)
      for (const [index, metadata] of manifest.entries()) {
        const file = files[index]!
        this.entries.set(metadata.fingerprint, {
          file,
          previewURL: this.pipeline.createURL(file),
          metadata,
          percent: 0,
          confirmed: false,
        })
      }
      this.publish('uploading')
      const reservation = await this.pipeline.reserve(this.projectId, manifest, controller.signal)
      if (!this.current(token)) {
        // An explicit cancel/replacement can beat reservation. If still on the page,
        // discard the newly learned id; an unloaded page leaves this to the server TTL.
        if (this.active) await this.pipeline.discard(reservation.batch.id)
        return
      }
      this.batchId = reservation.batch.id
      if (
        reservation.batch.projectId !== this.projectId ||
        reservation.batch.sources.length !== files.length ||
        reservation.uploads.length !== files.length
      )
        throw new Error('Invalid source reservation')
      let confirmed = reservation.batch
      for (const source of reservation.batch.sources) {
        const entry = this.entries.get(source.metadata.fingerprint)
        const upload = reservation.uploads.find((v) => v.sourceId === source.id)
        if (!entry || !upload) throw new Error('Missing reserved source')
        await this.pipeline.put(
          upload.putUrl,
          upload.headers,
          entry.file,
          (percent) => {
            if (!this.current(token)) return
            this.entries.set(source.metadata.fingerprint, { ...entry, percent })
            this.publish('uploading')
          },
          controller.signal,
        )
        if (!this.current(token)) return
        confirmed = await this.pipeline.confirm(reservation.batch.id, source.id, controller.signal)
        if (!this.current(token)) return
        this.entries.set(source.metadata.fingerprint, { ...entry, percent: 100, confirmed: true })
        this.publish('uploading')
      }
      if (confirmed.id !== reservation.batch.id || confirmed.state !== 'ready')
        throw new Error('Sources are not ready')
      this.publish('ready', { readyBatch: { ...confirmed, state: 'ready' } })
      this.controller = undefined
    } catch (error) {
      if (!this.current(token)) return
      const batchId = this.batchId
      if (batchId) {
        try {
          await this.pipeline.discard(batchId)
          if (this.current(token)) this.batchId = undefined
        } catch {
          /* Keep the id so an explicit retry can discard again. */
        }
      }
      if (this.current(token)) {
        this.clearFiles()
        this.publish('failed', { error })
      }
    }
  }
}
