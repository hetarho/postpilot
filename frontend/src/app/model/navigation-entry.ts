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
  if (!STABLE_PARENTS.has(url.pathname)) return undefined
  const filters: Record<string, string> = {}
  for (const [key, item] of url.searchParams)
    if (FILTER_KEYS.has(key) && item.length <= 512) filters[key] = item
  return { path: url.pathname + (url.hash.startsWith('#settings-') ? url.hash : ''), filters }
}
function store(ownerId: string, target: string, storage: NavigationStorage) {
  const scoped = {
    getItem: (key: string) => storage.getItem(`${key}.nav.${encodeURIComponent(target)}`),
    setItem: (key: string, value: string) =>
      storage.setItem(`${key}.nav.${encodeURIComponent(target)}`, value),
    removeItem: (key: string) => storage.removeItem(`${key}.nav.${encodeURIComponent(target)}`),
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
): boolean {
  const safe = navigationParent(entry)
  if (!ownerId || !storage || !safe || !safeInternalPath(target)) return false
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
  if (!storage || !ownerId || !safeInternalPath(target)) return undefined
  const entry = store(ownerId, target, storage).read()
  return entry?.targetId === target ? entry : undefined
}
export function entryHref(entry: ReturnContext): string {
  const [path, hash] = entry.path.split('#')
  const params = new URLSearchParams(entry.filters).toString()
  return `${path}${params ? '?' + params : ''}${hash ? '#' + hash : ''}`
}
