import {
  createReturnContextStore,
  type ReturnContext,
  type NavigationStorage,
} from '@/shared/lib/navigation'

export interface ClipEntry {
  path: '/' | '/clips' | '/library'
  section: string
  filters: Readonly<Record<string, string>>
  scrollY: number
  targetId?: string
}
export interface ClipReturnDestination {
  path: ClipEntry['path']
  href: string
  section: string
  scrollY: number
}
function browserStorage(): NavigationStorage | undefined {
  try {
    return window.sessionStorage
  } catch {
    return undefined
  }
}
/** Clip entries have their own namespace, independent of the current post/settings entry. */
function store(ownerId: string, storage: NavigationStorage, targetId?: string) {
  const suffix = `.clip.${targetId ? encodeURIComponent(targetId) : 'entry'}`
  return createReturnContextStore(
    ownerId,
    {
      getItem: (key) => storage.getItem(`${key}${suffix}`),
      setItem: (key, value) => storage.setItem(`${key}${suffix}`, value),
      removeItem: (key) => storage.removeItem(`${key}${suffix}`),
    },
    { isAccessible: (path) => ['/', '/clips', '/library'].includes(path) },
  )
}
export function rememberClipEntry(
  ownerId: string,
  entry: ClipEntry,
  storage = browserStorage(),
): boolean {
  if (!storage) return false
  const context: ReturnContext = {
    ...entry,
    version: 1,
    ownerKey: ownerId,
    targetId: entry.targetId ?? 'new',
    filters: { ...entry.filters },
  }
  const written = store(ownerId, storage, context.targetId).write(context)
  if (written) store(ownerId, storage).write(context)
  return written
}
export function readClipReturnContext(
  ownerId: string,
  clipId?: string,
  storage = browserStorage(),
): ReturnContext | undefined {
  if (!storage) return undefined
  const entry = store(ownerId, storage, clipId ?? 'new').read() ?? store(ownerId, storage).read()
  return entry?.targetId === (clipId ?? 'new') ? entry : undefined
}
export function clipReturnDestination(
  ownerId: string,
  clipId?: string,
  storage = browserStorage(),
): ClipReturnDestination {
  const entry = readClipReturnContext(ownerId, clipId, storage)
  const path = (entry?.path ?? (clipId ? '/clips' : '/')) as ClipEntry['path']
  const filters: Record<string, string> = {}
  if (entry?.filters.q) filters.q = entry.filters.q
  if (entry?.filters.status && ['draft', 'refining', 'finished'].includes(entry.filters.status))
    filters.status = entry.filters.status
  const query = new URLSearchParams(path === '/' ? {} : filters).toString()
  return {
    path,
    href: query ? `${path}?${query}` : path,
    section: entry?.section ?? (clipId ? 'clips' : 'creation'),
    scrollY: entry?.scrollY ?? 0,
  }
}
export function retainMintedClipEntry(
  ownerId: string,
  clipId: string,
  storage = browserStorage(),
  frozen?: ReturnContext | null,
): boolean {
  const source = frozen === undefined ? readClipReturnContext(ownerId, undefined, storage) : frozen
  const entry = source?.ownerKey === ownerId ? source : undefined
  return rememberClipEntry(
    ownerId,
    {
      path: (entry?.path ?? '/') as ClipEntry['path'],
      section: entry?.section ?? 'creation',
      filters: entry?.filters ?? {},
      scrollY: entry?.scrollY ?? 0,
      targetId: clipId,
    },
    storage,
  )
}
export function markClipHistoryReturn(
  ownerId: string,
  clipId: string,
  storage = browserStorage(),
): boolean {
  const entry = readClipReturnContext(ownerId, clipId, storage)
  if (!storage || !entry || entry.path !== '/clips') return false
  return store(ownerId, storage).write({
    ...entry,
    filters: { ...entry.filters, restoreScroll: 'true' },
  })
}
export function readClipHistoryReturn(
  ownerId: string,
  storage = browserStorage(),
): ReturnContext | undefined {
  if (!storage) return undefined
  const entry = store(ownerId, storage).read()
  return entry?.path === '/clips' && entry.filters.restoreScroll === 'true' ? entry : undefined
}
export function completeClipHistoryReturn(ownerId: string, storage = browserStorage()): void {
  const entry = readClipHistoryReturn(ownerId, storage)
  if (!storage || !entry) return
  const filters = Object.fromEntries(
    Object.entries(entry.filters).filter(([key]) => key !== 'restoreScroll'),
  )
  store(ownerId, storage).write({ ...entry, filters })
}
/** Confirmed deletion removes the record's origin while retaining a pending directory scroll. */
export function forgetClipEntry(ownerId: string, clipId: string, storage = browserStorage()): void {
  if (!storage) return
  store(ownerId, storage, clipId).clear()
  const current = store(ownerId, storage).read()
  if (current?.targetId !== clipId) return
  if (current.path === '/clips' && current.filters.restoreScroll === 'true') {
    const history = { ...current }
    delete history.targetId
    store(ownerId, storage).write(history)
  } else store(ownerId, storage).clear()
}
