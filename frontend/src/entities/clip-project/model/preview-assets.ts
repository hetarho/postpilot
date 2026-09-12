import { CLIP_DRAFT_PREVIEW, CLIP_COMPOSITION_LIMITS } from '@/shared/config'
import type { PreviewAsset, PreviewPage } from './draft-preview'

export type PreparedAsset = Omit<PreviewAsset, 'png'> & { url: string }
export interface PreviewAssetURLs {
  create: (bytes: Uint8Array) => string
  revoke: (url: string) => void
}

/** Runtime only. Nothing enters local storage or the query cache. */
export class PreviewAssetCache {
  private entries = new Map<string, { url: string; bytes: number }>()
  private bytes = 0
  constructor(
    private urls: PreviewAssetURLs,
    private limit: number = CLIP_DRAFT_PREVIEW.cacheBytes,
  ) {}
  put(asset: PreviewAsset): PreparedAsset {
    let entry = this.entries.get(asset.key)
    if (entry) this.entries.delete(asset.key)
    else {
      if (asset.png.byteLength > this.limit) throw new Error('CLIP_PREVIEW_TOO_LARGE')
      while (this.bytes + asset.png.byteLength > this.limit) {
        const oldest = this.entries.entries().next().value
        if (!oldest) break
        this.entries.delete(oldest[0])
        this.bytes -= oldest[1].bytes
        this.urls.revoke(oldest[1].url)
      }
      entry = { url: this.urls.create(asset.png), bytes: asset.png.byteLength }
      this.bytes += entry.bytes
    }
    this.entries.set(asset.key, entry)
    const { png, ...manifest } = asset
    void png
    return { ...manifest, url: entry.url }
  }
  clear() {
    for (const value of this.entries.values()) this.urls.revoke(value.url)
    this.entries.clear()
    this.bytes = 0
  }
}

export interface PreviewSnapshot {
  contentKey: string
  draftHash: string
  key: string
  status: 'idle' | 'updating' | 'ready' | 'failed'
  assets: PreparedAsset[]
  canvasWidth: number
  canvasHeight: number
  parity: readonly string[]
  error?: unknown
}
type LoadPage = (ids: string[], offset: number, signal: AbortSignal) => Promise<PreviewPage>
const EMPTY: PreviewSnapshot = {
  key: '',
  contentKey: '',
  draftHash: '',
  status: 'idle',
  assets: [],
  canvasWidth: 0,
  canvasHeight: 0,
  parity: [],
}

/** A response must match both the active request and the actual body hash.
 * Abort is advisory: identity guards also reject providers that ignore it. */
export class PreviewPreparation {
  private snapshot = EMPTY
  private listeners = new Set<() => void>()
  private controller?: AbortController
  private timer?: ReturnType<typeof setTimeout>
  constructor(private cache: PreviewAssetCache) {}
  getSnapshot = () => this.snapshot
  subscribe = (listener: () => void) => {
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }
  private publish(snapshot: PreviewSnapshot) {
    this.snapshot = snapshot
    this.listeners.forEach((fn) => fn())
  }
  update(key: string, hash: string, ids: string[], load: LoadPage, contentKey = key) {
    this.stop()
    const controller = new AbortController()
    this.controller = controller
    const retained =
      this.snapshot.contentKey === contentKey && this.snapshot.draftHash === hash
        ? this.snapshot
        : EMPTY
    this.publish({ ...retained, key, contentKey, draftHash: hash, status: 'updating' })
    this.timer = setTimeout(() => {
      void this.prepare(key, hash, ids, load, controller, contentKey)
    }, CLIP_DRAFT_PREVIEW.debounceMs)
  }
  private async prepare(
    key: string,
    hash: string,
    ids: string[],
    load: LoadPage,
    controller: AbortController,
    contentKey: string,
  ) {
    try {
      const groups: string[][] = []
      for (let i = 0; i < ids.length; i += CLIP_DRAFT_PREVIEW.maxAssets)
        groups.push(ids.slice(i, i + CLIP_DRAFT_PREVIEW.maxAssets))
      if (!groups.length) groups.push([])
      const assets: PreviewAsset[] = []
      let page: PreviewPage | undefined
      const unique = new Map<string, number>()
      let pages = 0
      for (const group of groups) {
        let offset = 0
        do {
          if (
            ++pages > CLIP_COMPOSITION_LIMITS.cues ||
            assets.length > CLIP_COMPOSITION_LIMITS.cues
          )
            throw new Error('CLIP_PREVIEW_TOO_LARGE')
          page = await load(group, offset, controller.signal)
          if (this.controller !== controller || controller.signal.aborted) return
          if (page.draftHash !== hash) throw new Error('CLIP_PREVIEW_STALE')
          if (
            page.assets.length > CLIP_DRAFT_PREVIEW.maxAssets ||
            page.assets.some((a) => a.png.byteLength > CLIP_DRAFT_PREVIEW.maxAssetBytes) ||
            page.assets.reduce((sum, a) => sum + a.png.byteLength, 0) >
              CLIP_DRAFT_PREVIEW.maxResponseBytes
          )
            throw new Error('CLIP_PREVIEW_TOO_LARGE')
          for (const asset of page.assets) unique.set(asset.key, asset.png.byteLength)
          if (
            [...unique.values()].reduce((sum, bytes) => sum + bytes, 0) >
            CLIP_DRAFT_PREVIEW.cacheBytes
          )
            throw new Error('CLIP_PREVIEW_TOO_LARGE')
          assets.push(...page.assets)
          if (page.nextOffset !== -1 && page.nextOffset <= offset)
            throw new Error('CLIP_PREVIEW_INVALID')
          offset = page.nextOffset
        } while (offset !== -1)
      }
      if (!page || this.controller !== controller || controller.signal.aborted) return
      this.publish({
        key,
        contentKey,
        draftHash: hash,
        status: 'ready',
        assets: assets.map((a) => this.cache.put(a)),
        canvasWidth: page.canvasWidth,
        canvasHeight: page.canvasHeight,
        parity: page.parity,
      })
    } catch (error) {
      if (this.controller === controller && !controller.signal.aborted)
        this.publish({ ...EMPTY, key, contentKey, draftHash: hash, status: 'failed', error })
    }
  }
  stop() {
    clearTimeout(this.timer)
    this.controller?.abort()
    this.controller = undefined
  }
  dispose() {
    this.stop()
    this.cache.clear()
    this.publish(EMPTY)
  }
}
