import { CLIP_INK } from '@/entities/clip-design/@x/clip-preview'
import { ClipInkError, inkRasterDimensions } from './ink-typography'
import type { InkDocument } from './ink-raster'

interface GPUResource {
  value: unknown
  destroy: () => void
}
interface Entry {
  key: string
  bytes: number
  refs: number
  pending: boolean
  promise: Promise<ImageBitmap>
  bitmap?: ImageBitmap
  gpu: Map<object, GPUResource>
  phase: { x: number; y: number }
}
export interface BrowserInkLease {
  readonly bitmap: ImageBitmap
  readonly offset: { x: number; y: number }
  gpu<T>(owner: object, create: () => { value: T; destroy: () => void }): T
  close(): void
}

/** Reservations, serial rasterization and live leases bound CPU/GPU ink together. */
export class BrowserInkCache {
  private entries = new Map<string, Entry>()
  private controller = new AbortController()
  private tail: Promise<unknown> = Promise.resolve()
  private bytes = 0
  private peakBytes = 0
  private rasters = 0
  constructor(
    private render: (document: InkDocument, signal: AbortSignal) => Promise<ImageBitmap>,
    private budget: { bytes: number; entries: number } = {
      bytes: CLIP_INK.cacheBytes,
      entries: CLIP_INK.cacheEntries,
    },
  ) {}
  private discard(entry: Entry) {
    for (const resource of entry.gpu.values()) resource.destroy()
    entry.gpu.clear()
    entry.bitmap?.close()
    if (this.entries.get(entry.key) === entry) {
      this.entries.delete(entry.key)
      this.bytes -= entry.bytes
    }
  }
  private capacity(bytes: number) {
    for (const entry of this.entries.values()) {
      if (this.bytes + bytes <= this.budget.bytes && this.entries.size < this.budget.entries) break
      if (!entry.refs && !entry.pending) this.discard(entry)
    }
    if (
      bytes > this.budget.bytes ||
      this.bytes + bytes > this.budget.bytes ||
      this.entries.size >= this.budget.entries
    )
      throw new ClipInkError('CLIP_INK_RESOURCE_LIMIT')
  }
  async acquire(document: InkDocument, signal?: AbortSignal): Promise<BrowserInkLease> {
    if (signal?.aborted || this.controller.signal.aborted)
      throw new ClipInkError('CLIP_INK_CANCELLED')
    const dimensions = inkRasterDimensions(document.bounds, document.rasterScale)
    let entry = this.entries.get(document.key)
    if (!entry) {
      const bytes = dimensions.width * dimensions.height * 4
      this.capacity(bytes)
      const created: Entry = {
        key: document.key,
        bytes,
        refs: 0,
        pending: true,
        promise: Promise.resolve(undefined as unknown as ImageBitmap),
        gpu: new Map(),
        phase: document.phase ?? { x: 0, y: 0 },
      }
      this.entries.set(document.key, created)
      this.bytes += bytes
      this.peakBytes = Math.max(this.peakBytes, this.bytes)
      created.promise = this.tail
        .then(async () => {
          if (this.controller.signal.aborted) throw new ClipInkError('CLIP_INK_CANCELLED')
          const bitmap = await this.render(document, this.controller.signal)
          if (this.controller.signal.aborted || this.entries.get(document.key) !== created) {
            bitmap.close()
            throw new ClipInkError('CLIP_INK_CANCELLED')
          }
          if (bitmap.width !== dimensions.width || bitmap.height !== dimensions.height) {
            bitmap.close()
            throw new ClipInkError('CLIP_INK_INVALID_GEOMETRY', 'raster dimensions')
          }
          created.bitmap = bitmap
          created.pending = false
          this.rasters++
          return bitmap
        })
        .catch((error) => {
          this.discard(created)
          throw error
        })
      this.tail = created.promise.catch(() => undefined)
      entry = created
    }
    const bitmap = await entry.promise
    if (signal?.aborted || this.controller.signal.aborted || this.entries.get(entry.key) !== entry)
      throw new ClipInkError('CLIP_INK_CANCELLED')
    this.entries.delete(entry.key)
    this.entries.set(entry.key, entry)
    entry.refs++
    let closed = false
    return {
      bitmap,
      offset: {
        x: (document.phase?.x ?? 0) - entry.phase.x,
        y: (document.phase?.y ?? 0) - entry.phase.y,
      },
      gpu: <T>(owner: object, create: () => { value: T; destroy: () => void }) => {
        if (closed || this.controller.signal.aborted) throw new ClipInkError('CLIP_INK_CANCELLED')
        let resource = entry.gpu.get(owner)
        if (!resource) {
          // Each run owns one compositor; another owner is a lifecycle error,
          // not permission to accumulate one texture per frame.
          if (entry.gpu.size) throw new ClipInkError('CLIP_INK_RESOURCE_LIMIT', 'GPU owner')
          resource = create()
          entry.gpu.set(owner, resource)
        }
        return resource.value as T
      },
      close: () => {
        if (!closed) {
          closed = true
          entry.refs--
        }
      },
    }
  }
  dropGPU(owner: object) {
    for (const entry of this.entries.values()) {
      entry.gpu.get(owner)?.destroy()
      entry.gpu.delete(owner)
    }
  }
  measurements() {
    return {
      entries: this.entries.size,
      bytes: this.bytes,
      peakBytes: this.peakBytes,
      rasters: this.rasters,
      leases: [...this.entries.values()].reduce((n, entry) => n + entry.refs, 0),
    }
  }
  destroy() {
    this.controller.abort()
    for (const entry of [...this.entries.values()]) this.discard(entry)
  }
}
