import {
  createReturnContextStore,
  type ReturnContext,
  type NavigationStorage,
} from '@/shared/lib/navigation'

export interface PostEntry {
  /** The list the post was opened from: the channel's own, or 내 글 전체 (THEME-69). */
  path: '/naver-blog' | '/my'
  section: string
  filters: Readonly<Record<string, string>>
  scrollY: number
  targetId?: string
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
    isAccessible: (path) => ['/naver-blog', '/posts', '/', '/library', '/my'].includes(path),
  })
}

/** The channel opens on its own history (POST-43), so history and creation share one address and
 *  the section tells them apart. A `/posts` entry stored before that still names the history. */
function isPostHistoryEntry(entry: Pick<ReturnContext, 'path' | 'section'>): boolean {
  return entry.path !== '/my' && (entry.section === 'posts' || entry.path === '/posts')
}

/** Work opened from 내 글 전체 returns there, with its scroll (THEME-58 →THEME-69). */
function isMyWorkEntry(entry: Pick<ReturnContext, 'path'>): boolean {
  return entry.path === '/my'
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
  if (!entry || entry.targetId !== (slug ?? 'new')) return undefined
  if (entry.path === '/' || entry.path === '/library')
    return { ...entry, path: '/naver-blog', section: 'naverBlog', filters: {} }
  return entry.path === '/posts' ? { ...entry, path: '/naver-blog', section: 'posts' } : entry
}

export function postReturnDestination(ownerId: string, slug?: string, storage = browserStorage()) {
  const entry = readPostReturnContext(ownerId, slug, storage)
  if (entry && isMyWorkEntry(entry))
    return { path: '/my' as const, href: '/my', section: entry.section, scrollY: entry.scrollY }
  const path = '/naver-blog' as const
  const search = Object.fromEntries(
    Object.entries(entry?.filters ?? {}).filter(([key]) => key === 'q' || key === 'status'),
  )
  const query = new URLSearchParams(search).toString()
  return {
    path,
    href: query ? `${path}?${query}` : path,
    section: entry?.section ?? (slug ? 'posts' : 'naverBlog'),
    scrollY: entry?.scrollY ?? 0,
  }
}

export function retainMintedPostEntry(ownerId: string, slug: string) {
  const entry = readPostReturnContext(ownerId)
  return rememberPostEntry(ownerId, {
    path: entry && isMyWorkEntry(entry) ? '/my' : '/naver-blog',
    section: entry?.section ?? 'naverBlog',
    filters: entry?.filters ?? {},
    scrollY: entry?.scrollY ?? 0,
    targetId: slug,
  })
}

export function markPostHistoryReturn(ownerId: string, slug: string) {
  const storage = browserStorage()
  const entry = readPostReturnContext(ownerId, slug, storage)
  if (!storage || !entry || !(isPostHistoryEntry(entry) || isMyWorkEntry(entry))) return false
  return store(ownerId, storage).write({
    ...entry,
    filters: { ...entry.filters, restoreScroll: 'true' },
  })
}

export function readPostHistoryReturn(ownerId: string) {
  const storage = browserStorage()
  if (!storage) return undefined
  const entry = store(ownerId, storage).read()
  return entry && isPostHistoryEntry(entry) && entry.filters.restoreScroll === 'true'
    ? entry
    : undefined
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

/** A return to 내 글 전체 that has yet to restore its scroll, marked by `markPostHistoryReturn`. */
export function readMyWorkPostReturn(ownerId: string) {
  const storage = browserStorage()
  if (!storage) return undefined
  const entry = store(ownerId, storage).read()
  return entry && isMyWorkEntry(entry) && entry.filters.restoreScroll === 'true' ? entry : undefined
}

export function completeMyWorkPostReturn(ownerId: string) {
  const storage = browserStorage()
  const entry = readMyWorkPostReturn(ownerId)
  if (!storage || !entry) return
  const filters = Object.fromEntries(
    Object.entries(entry.filters).filter(([key]) => key !== 'restoreScroll'),
  )
  store(ownerId, storage).write({ ...entry, filters })
}
