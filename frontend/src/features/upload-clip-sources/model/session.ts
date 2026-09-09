import type { ClipSourceBatch, ClipSourceMetadata, ReadyClipBatch } from '@/entities/clip-project'

export interface SourcePipeline {
  read(files: readonly File[], signal: AbortSignal): Promise<ClipSourceMetadata[]>
  reserve(
    projectId: string,
    manifest: ClipSourceMetadata[],
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
  confirm(batchId: string, sourceId: string): Promise<ClipSourceBatch>
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
  phase: 'idle' | 'reading' | 'uploading' | 'ready' | 'cancelling' | 'failed'
  entries: readonly LocalClipSource[]
  readyBatch?: ReadyClipBatch
  error?: unknown
}

/** A page-owned session, never a singleton or a query-cache value. */
export class ClipSourceSession {
  private active = false
  private epoch = 0
  private controller?: AbortController
  private batchId?: string
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
  }
  private publish(
    phase: ClipUploadState['phase'],
    extras: Pick<ClipUploadState, 'readyBatch' | 'error'> = {},
  ) {
    this.state = { phase, entries: [...this.entries.values()], ...extras }
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
    this.epoch++
    this.controller?.abort()
    this.clearFiles()
    this.batchId = undefined
    this.publish('idle')
    // No asynchronous discard from unmount/unload. The server's lease and orphan sweep own it.
  }
  /** Called when a processing attempt ends; its worker owns remote cleanup. */
  finishAttempt() {
    this.epoch++
    this.controller?.abort()
    this.clearFiles()
    this.batchId = undefined
    this.publish('idle')
  }
  async cancel() {
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
  async select(files: readonly File[]) {
    if (!files.length || !this.active) return
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
      const reservation = await this.pipeline.reserve(this.projectId, manifest)
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
        confirmed = await this.pipeline.confirm(reservation.batch.id, source.id)
        if (!this.current(token)) return
        this.entries.set(source.metadata.fingerprint, { ...entry, percent: 100, confirmed: true })
        this.publish('uploading')
      }
      if (confirmed.id !== reservation.batch.id || confirmed.state !== 'ready')
        throw new Error('Sources are not ready')
      this.publish('ready', { readyBatch: { ...confirmed, state: 'ready' } })
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
