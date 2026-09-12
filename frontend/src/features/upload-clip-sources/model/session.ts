import { appFailureFromConnect } from '@/shared/api'
import type {
  ClipSourceBatch,
  ClipSourceMetadata,
  ReadyClipBatch,
  ClipSourceAvailability,
} from '@/entities/clip-project'

export interface SourcePipeline {
  retained?(projectId: string, signal?: AbortSignal): Promise<ClipSourceBatch[]>
  playback?(
    projectId: string,
    id: string,
    fingerprint: string,
    signal?: AbortSignal,
  ): Promise<{ url: string; expiresAt: string }>
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
  file?: File
  sourceId?: string
  retentionExpiresAt?: string
  playbackExpiresAt?: string
  availability?: ClipSourceAvailability
  playbackError?: 'expired' | 'missing' | 'unavailable'
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

export class ClipSourceAccessError extends Error {
  constructor(public readonly reason: 'expired' | 'missing' | 'unavailable') {
    super(`Clip source ${reason}`)
  }
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
  private mediaEpoch = 0
  private playbackController = new AbortController()
  private controller?: AbortController
  private batchId?: string
  private requiredFingerprints?: readonly string[]
  private lastFinishedJob?: string
  private refreshing?: Promise<void>
  private playbackRequests = new Map<string, Promise<string>>()
  private playbackRetried = new Set<string>()
  private retainedController?: AbortController
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
    if (this.playbackController.signal.aborted) this.playbackController = new AbortController()
    activeSessions.add(this)
    void this.refreshRetained()
  }
  requireSources(fingerprints: readonly string[] | undefined) {
    this.requiredFingerprints = fingerprints
    void this.refreshRetained()
  }

  refreshRetained = (): Promise<void> => {
    if (!this.active || !this.pipeline.retained || this.attempt || this.controller)
      return Promise.resolve()
    if (this.refreshing) return this.refreshing
    const epoch = this.epoch
    const controller = new AbortController()
    this.retainedController?.abort()
    this.retainedController = controller
    const request = (async () => {
      try {
        const batches = await this.pipeline.retained!(this.projectId, controller.signal)
        if (!this.current(epoch) || this.attempt) return
        const current = batches.find((b) => b.current)
        const seen = new Set<string>()
        for (const batch of batches)
          for (const source of batch.sources) {
            const fp = source.metadata.fingerprint
            if (seen.has(fp)) continue
            seen.add(fp)
            const prior = this.entries.get(fp)
            this.entries.set(fp, {
              ...prior,
              metadata: source.metadata,
              sourceId: source.id,
              previewURL:
                prior?.file || prior?.sourceId === source.id ? (prior?.previewURL ?? '') : '',
              percent: source.state === 'ready' ? 100 : 0,
              confirmed: source.state === 'ready',
              retentionExpiresAt: source.retentionExpiresAt,
              availability: source.availability,
            })
          }
        for (const [fp, entry] of this.entries)
          if (!seen.has(fp) && !entry.file) this.entries.delete(fp)
        const needed =
          this.requiredFingerprints ?? current?.sources.map((s) => s.metadata.fingerprint) ?? []
        const ready =
          current?.state === 'ready' &&
          needed.length > 0 &&
          needed.every((fp) =>
            current.sources.some(
              (source) =>
                source.metadata.fingerprint === fp &&
                source.availability === 'available' &&
                !!source.retentionExpiresAt &&
                Date.parse(source.retentionExpiresAt) > Date.now(),
            ),
          )
        this.batchId = current?.id
        this.publish(
          ready
            ? 'ready'
            : current?.state === 'consuming' && !!this.lastFinishedJob
              ? 'finished'
              : 'idle',
          {
            ...(ready ? { readyBatch: { ...current, state: 'ready' } } : {}),
            summaries: this.state.summaries,
          },
        )
      } catch (error) {
        if (this.current(epoch) && !controller.signal.aborted)
          this.publish(this.state.phase, { readyBatch: this.state.readyBatch, error })
      }
    })().finally(() => {
      if (this.refreshing === request) this.refreshing = undefined
    })
    this.refreshing = request
    return request
  }
  ensurePlayback = (fingerprint: string, refresh = false): Promise<string> => {
    const entry = this.entries.get(fingerprint)
    if (!this.active || !entry) return Promise.reject(new ClipSourceAccessError('unavailable'))
    if (entry.file && entry.previewURL) return Promise.resolve(entry.previewURL)
    if (this.playbackRequests.has(fingerprint)) return this.playbackRequests.get(fingerprint)!
    if (
      !refresh &&
      entry.previewURL &&
      entry.playbackExpiresAt &&
      Date.parse(entry.playbackExpiresAt) > Date.now()
    )
      return Promise.resolve(entry.previewURL)
    const epoch = this.mediaEpoch
    const currentMedia = () => this.active && epoch === this.mediaEpoch
    const request = (async () => {
      try {
        if (!entry.sourceId || !this.pipeline.playback)
          throw new ClipSourceAccessError('unavailable')
        if (
          !entry.retentionExpiresAt ||
          Date.parse(entry.retentionExpiresAt) <= Date.now() ||
          ['expired', 'cleanup_pending'].includes(entry.availability ?? '')
        )
          throw new ClipSourceAccessError('expired')
        if (entry.availability === 'missing') throw new ClipSourceAccessError('missing')
        if (refresh) {
          if (this.playbackRetried.has(fingerprint)) throw new ClipSourceAccessError('unavailable')
          this.playbackRetried.add(fingerprint)
        }
        const capability = await this.pipeline.playback(
          this.projectId,
          entry.sourceId,
          fingerprint,
          this.playbackController.signal,
        )
        if (!currentMedia()) throw new ClipSourceAccessError('unavailable')
        const current = this.entries.get(fingerprint)
        if (!current || current.sourceId !== entry.sourceId)
          throw new ClipSourceAccessError('unavailable')
        this.entries.set(fingerprint, {
          ...current,
          previewURL: capability.url,
          playbackExpiresAt: capability.expiresAt,
          playbackError: undefined,
        })
        this.publish(this.state.phase, {
          readyBatch: this.state.readyBatch,
          summaries: this.state.summaries,
        })
        return capability.url
      } catch (error) {
        const failure = appFailureFromConnect(error)
        const reason =
          error instanceof ClipSourceAccessError
            ? error.reason
            : failure.reason === 'CLIP_SOURCE_EXPIRED'
              ? 'expired'
              : failure.reason === 'CLIP_SOURCE_MISSING'
                ? 'missing'
                : 'unavailable'
        if (currentMedia()) {
          this.entries.set(fingerprint, { ...entry, previewURL: '', playbackError: reason })
          this.publish(this.state.phase, {
            readyBatch: this.state.readyBatch,
            summaries: this.state.summaries,
          })
        }
        throw new ClipSourceAccessError(reason)
      }
    })().finally(() => {
      if (this.playbackRequests.get(fingerprint) === request)
        this.playbackRequests.delete(fingerprint)
    })
    this.playbackRequests.set(fingerprint, request)
    return request
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
    this.mediaEpoch++
    this.playbackController.abort()
    this.playbackController = new AbortController()
    this.playbackRequests.clear()
    for (const entry of this.entries.values())
      if (entry.file) this.pipeline.revokeURL(entry.previewURL)
    this.playbackRetried.clear()
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
    this.retainedController?.abort()
    this.refreshing = undefined
    this.clearFiles()
    this.playbackController.abort()
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
    if (!this.active) return
    if (!this.attempt) {
      if (this.lastFinishedJob !== jobId) {
        this.lastFinishedJob = jobId
        void this.refreshRetained()
      }
      return
    }
    if (this.attempt.jobId !== jobId || this.attempt.epoch !== this.epoch) return
    this.lastFinishedJob = jobId
    const summaries = [...this.entries.values()].map((e) => ({
      filename: e.metadata.filename,
      status,
    }))
    this.epoch++
    this.controller?.abort()
    this.controller = undefined
    this.batchId = this.attempt.batch.id
    this.attempt = undefined
    this.publish('finished', { summaries })
    void this.refreshRetained()
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
    this.clearFiles()
    this.publish('reading')
    try {
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
        if (!entry?.file || !upload) throw new Error('Missing reserved source')
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
        const lease = confirmed.sources.find((v) => v.id === source.id)
        this.entries.set(source.metadata.fingerprint, {
          ...entry,
          sourceId: source.id,
          percent: 100,
          confirmed: true,
          retentionExpiresAt: lease?.retentionExpiresAt,
          availability: lease?.availability,
        })
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
