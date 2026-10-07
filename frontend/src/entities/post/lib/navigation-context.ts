import {
  createReturnContextStore,
  type ReturnContext,
  type NavigationStorage,
} from '@/shared/lib/navigation'

export interface PostEntry {
  path: '/' | '/posts' | '/library'
  section: string
  filters: Readonly<Record<string, string>>
  scrollY: number
  targetId?: string
  intent?: 'export'
}

// Only stable owned parents are eligible. A deleted record, foreign owner or stale
// entry for a different post cannot become the editor's return destination.
function store(ownerId: string, storage: NavigationStorage, targetId?: string) {
  const scoped = targetId
    ? {
        getItem: (key: string) => storage.getItem(`${key}.post.${encodeURIComponent(targetId)}`),
        setItem: (key: string, value: string) =>
          storage.setItem(`${key}.post.${encodeURIComponent(targetId)}`, value),
        removeItem: (key: string) =>
          storage.removeItem(`${key}.post.${encodeURIComponent(targetId)}`),
      }
    : storage
  return createReturnContextStore(ownerId, scoped, {
    isAccessible: (path) => ['/', '/posts', '/library'].includes(path),
  })
}

function browserStorage(): NavigationStorage | undefined {
  try {
    return window.sessionStorage
  } catch {
    return undefined
  }
}

export function rememberPostEntry(
  ownerId: string,
  entry: PostEntry,
  storage = browserStorage(),
): boolean {
  if (!storage) return false
  const context: ReturnContext = {
    ...entry,
    version: 1,
    ownerKey: ownerId,
    targetId: entry.targetId ?? 'new',
    filters: { ...entry.filters, ...(entry.intent ? { intent: entry.intent } : {}) },
  }
  // Children read the generic current entry; each post also retains its own copy
  // when another record replaces that current navigation context.
  store(ownerId, storage).write(context)
  return store(ownerId, storage, context.targetId).write(context)
}

export function readPostReturnContext(
  ownerId: string,
  slug?: string,
  storage = browserStorage(),
): ReturnContext | undefined {
  if (!storage) return undefined
  const entry = store(ownerId, storage, slug ?? 'new').read() ?? store(ownerId, storage).read()
  return entry && entry.targetId === (slug ?? 'new') ? entry : undefined
}

export function postReturnDestination(ownerId: string, slug?: string, storage = browserStorage()) {
  const entry = readPostReturnContext(ownerId, slug, storage)
  const path = (entry?.path ?? (slug ? '/posts' : '/')) as PostEntry['path']
  const search = Object.fromEntries(
    Object.entries(entry?.filters ?? {}).filter(([key]) => key === 'q' || key === 'status'),
  )
  const query = new URLSearchParams(search).toString()
  return {
    path,
    href: query ? `${path}?${query}` : path,
    section: entry?.section ?? (slug ? 'posts' : 'creation'),
    scrollY: entry?.scrollY ?? 0,
  }
}

export function retainMintedPostEntry(ownerId: string, slug: string) {
  const entry = readPostReturnContext(ownerId)
  return rememberPostEntry(ownerId, {
    path: (entry?.path ?? '/') as PostEntry['path'],
    section: entry?.section ?? 'creation',
    filters: entry?.filters ?? {},
    scrollY: entry?.scrollY ?? 0,
    targetId: slug,
  })
}

export function markPostHistoryReturn(ownerId: string, slug: string) {
  const storage = browserStorage()
  const entry = readPostReturnContext(ownerId, slug, storage)
  if (!storage || !entry || entry.path !== '/posts') return false
  return store(ownerId, storage).write({
    ...entry,
    filters: { ...entry.filters, restoreScroll: 'true' },
  })
}

export function readPostHistoryReturn(ownerId: string) {
  const storage = browserStorage()
  if (!storage) return undefined
  const entry = store(ownerId, storage).read()
  return entry?.path === '/posts' && entry.filters.restoreScroll === 'true' ? entry : undefined
}

export function completePostHistoryReturn(ownerId: string) {
  const storage = browserStorage()
  const entry = readPostHistoryReturn(ownerId)
  if (!storage || !entry) return
  const filters = Object.fromEntries(
    Object.entries(entry.filters).filter(([key]) => key !== 'restoreScroll'),
  )
  store(ownerId, storage).write({ ...entry, filters })
}
