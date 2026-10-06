import type { BrowserMediaSourceAccess } from '@/shared/lib'
import { BrowserCompositionError, type BrowserCompositionSnapshot } from './browser-composition'

export interface BrowserLocalOriginal {
  sourceId?: string
  fingerprint: string
  file?: File
  url?: string
}
/** One owner/project-bound runtime access cache. It serializes no URL or original bytes. */
export class BrowserCompositionOriginals {
  private accesses = new Map<string, Promise<BrowserMediaSourceAccess>>()
  private disposed = false
  private refreshAccess = false
  constructor(
    private readonly snapshot: BrowserCompositionSnapshot,
    private readonly local: readonly BrowserLocalOriginal[],
    private readonly resolvePlayback: (fingerprint: string, refresh?: boolean) => Promise<string>,
    private readonly signal: AbortSignal,
  ) {}
  source = async (
    sourceId: string,
    fingerprint: string,
    signal: AbortSignal,
  ): Promise<BrowserMediaSourceAccess> => {
    this.check(signal)
    const identity = this.snapshot.sources.find(
      (source) => source.sourceId === sourceId && source.fingerprint === fingerprint,
    )
    if (!identity) throw new BrowserCompositionError('CLIP_SOURCE_IDENTITY_MISMATCH', sourceId)
    const key = JSON.stringify([
      this.snapshot.ownerId,
      this.snapshot.projectId,
      sourceId,
      fingerprint,
    ])
    let access = this.accesses.get(key)
    if (!access) {
      access = (async () => {
        const selected = this.local.find(
          (source) => source.sourceId === sourceId && source.fingerprint === fingerprint,
        )
        if (selected?.file) return { kind: 'blob' as const, blob: selected.file }
        const url = selected?.url?.startsWith('blob:')
          ? selected.url
          : await this.resolvePlayback(fingerprint, this.refreshAccess)
        this.check(signal)
        return { kind: 'url' as const, url }
      })()
      this.accesses.set(key, access)
      void access.catch(() => {
        if (this.accesses.get(key) === access) this.accesses.delete(key)
      })
    }
    const resolved = await access
    this.check(signal)
    return resolved
  }
  audioSource = (fingerprint: string, signal: AbortSignal) => {
    const source = this.snapshot.sources.find((source) => source.fingerprint === fingerprint)
    if (!source) throw new BrowserCompositionError('CLIP_SOURCE_IDENTITY_MISMATCH')
    return this.source(source.sourceId, fingerprint, signal)
  }
  refresh() {
    this.accesses.clear()
    this.refreshAccess = true
  }
  private check(signal: AbortSignal) {
    this.signal.throwIfAborted()
    signal.throwIfAborted()
    if (this.disposed) throw new BrowserCompositionError('CLIP_SNAPSHOT_SUPERSEDED')
  }
  dispose() {
    this.disposed = true
    this.accesses.clear()
  }
}
