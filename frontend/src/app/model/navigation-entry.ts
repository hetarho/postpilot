import {
  isStructuralNavigationEntry,
  isTopLevelDestination,
  navigationPathname,
} from '../routes/navigation'
import {
  createReturnContextStore,
  safeInternalPath,
  type NavigationStorage,
  type ReturnContext,
} from '@/shared/lib/navigation'

const STABLE_PARENTS = new Set([
  '/',
  '/settings',
  '/library',
  '/posts',
  '/clips',
  '/tests',
  '/tests/history',
  '/voices',
  '/templates',
  '/guidelines',
  '/memories',
  '/video-templates',
  '/video-guidelines',
  '/spoken-voices',
  '/ai-models',
  '/account',
  '/plans',
  '/billing',
  '/admin',
])
const FILTER_KEYS = new Set(['q', 'status', 'stage', 'source', 'voiceId', 'factor', 'count', 'tab'])
function browserStorage(): NavigationStorage | undefined {
  try {
    return globalThis.sessionStorage
  } catch {
    return undefined
  }
}
export function navigationParent(
  value: string,
): { path: string; filters: Record<string, string> } | undefined {
  if (!safeInternalPath(value)) return undefined
  const url = new URL(value, 'https://postpilot.invalid')
  const pathname = navigationPathname(url.pathname)
  if (!STABLE_PARENTS.has(pathname)) return undefined
  const filters: Record<string, string> = {}
  for (const [key, item] of url.searchParams)
    if (FILTER_KEYS.has(key) && item.length <= 512) filters[key] = item
  const hash =
    pathname === '/settings' &&
    ['#settings-writing', '#settings-video', '#settings-ai'].includes(url.hash)
      ? url.hash
      : ''
  return { path: pathname + hash, filters }
}
function store(ownerId: string, target: string, storage: NavigationStorage) {
  const scoped = {
    getItem: (key: string) => storage.getItem(`${key}.nav.v2.${encodeURIComponent(target)}`),
    setItem: (key: string, value: string) =>
      storage.setItem(`${key}.nav.v2.${encodeURIComponent(target)}`, value),
    removeItem: (key: string) => storage.removeItem(`${key}.nav.v2.${encodeURIComponent(target)}`),
  }
  return createReturnContextStore(ownerId, scoped, {
    isAccessible: (path) => !!navigationParent(path),
  })
}
export function rememberNavigationEntry(
  ownerId: string,
  target: string,
  entry: string,
  section: string,
  scrollY: number,
  storage = browserStorage(),
  explicit = false,
): boolean {
  const safe = navigationParent(entry)
  if (!ownerId || !storage || !safe || !safeInternalPath(target)) return false
  const targetPath = navigationPathname(target)
  if (isTopLevelDestination(targetPath) || safe.path.split('#', 1)[0] === targetPath) return false
  if (!explicit && !isStructuralNavigationEntry(target, entry)) return false
  return store(ownerId, target, storage).write({
    version: 1,
    ownerKey: ownerId,
    ...safe,
    section,
    scrollY: Math.max(0, scrollY),
    targetId: target,
  })
}
export function readNavigationEntry(
  ownerId: string,
  target: string,
  storage = browserStorage(),
): ReturnContext | undefined {
  if (
    !storage ||
    !ownerId ||
    !safeInternalPath(target) ||
    isTopLevelDestination(target.split(/[?#]/u, 1)[0])
  )
    return undefined
  const entry = store(ownerId, target, storage).read()
  return entry?.targetId === target ? entry : undefined
}
export function entryHref(entry: Pick<ReturnContext, 'path' | 'filters'>): string {
  const [path, hash] = entry.path.split('#')
  const filters = new URLSearchParams()
  for (const key of FILTER_KEYS)
    if (entry.filters[key] !== undefined) filters.set(key, entry.filters[key])
  const params = filters.toString()
  return `${path}${params ? '?' + params : ''}${hash ? '#' + hash : ''}`
}
