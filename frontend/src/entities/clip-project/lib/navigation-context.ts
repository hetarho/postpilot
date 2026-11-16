import {
  createReturnContextStore,
  type ReturnContext,
  type NavigationStorage,
} from '@/shared/lib/navigation'

import { CLIP_CHANNELS, DEFAULT_CLIP_CHANNEL, type ClipChannel } from '../config/channel'
import { clipHistoryFilter } from '../model/history-state'

export interface ClipEntry {
  /** The list the clip was opened from: the 영상 list, or 내 콘텐츠 전체 (THEME-69). */
  path: '/clips' | '/my'
  section: string
  filters: Readonly<Record<string, string>>
  scrollY: number
  targetId?: string
}
/** The one 영상 list (CLIP-41); a video has no channel address of its own. */
export const CLIP_LIST_PATH = '/clips'
/** Addresses the list answered to before CLIP-41; a context stored under one still returns here. */
const LEGACY_CLIP_LIST_PATHS = ['/naver-clip', '/insta-clip', '/library', '/']
export interface ClipReturnDestination {
  channel: ClipChannel
  path: ClipEntry['path']
  href: string
  section: string
  scrollY: number
}
/** The 영상 list opens on its own history (CLIP-41), so history and creation share one address
 *  and the section tells them apart. */
export function isClipHistoryEntry(entry: Pick<ReturnContext, 'path' | 'section'>): boolean {
  return entry.path !== '/my' && entry.section === 'clips'
}
/** Work opened from 내 글 전체 returns there, with its scroll (THEME-58 →THEME-69). */
function isMyWorkEntry(entry: Pick<ReturnContext, 'path'>): boolean {
  return entry.path === '/my'
}
function storedChannel(entry: ReturnContext | undefined): ClipChannel {
  return CLIP_CHANNELS.includes(entry?.filters.channel as ClipChannel)
    ? (entry!.filters.channel as ClipChannel)
    : DEFAULT_CLIP_CHANNEL
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
    {
      isAccessible: (path) => [CLIP_LIST_PATH, ...LEGACY_CLIP_LIST_PATHS, '/my'].includes(path),
    },
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
  if (entry?.targetId !== (clipId ?? 'new')) return undefined
  if (!LEGACY_CLIP_LIST_PATHS.includes(entry.path)) return entry
  // An Instagram list kept its channel in the address; the channel now rides in the filters.
  const filters =
    entry.path === '/insta-clip' && !entry.filters.channel
      ? { ...entry.filters, channel: 'instagram_clip' }
      : entry.filters
  return { ...entry, path: CLIP_LIST_PATH, filters }
}
export function clipReturnDestination(
  ownerId: string,
  clipId?: string,
  storage = browserStorage(),
  channel?: ClipChannel,
): ClipReturnDestination {
  const entry = readClipReturnContext(ownerId, clipId, storage)
  const currentChannel = channel ?? storedChannel(entry)
  if (entry && isMyWorkEntry(entry))
    return {
      channel: currentChannel,
      path: '/my',
      href: '/my',
      section: entry.section,
      scrollY: entry.scrollY,
    }
  // A history entry adds its own narrowing to the one list's address.
  const path = CLIP_LIST_PATH
  const history = entry ? isClipHistoryEntry(entry) : !!clipId
  const filters: Record<string, string> = {}
  if (history) {
    if (entry?.filters.q) filters.q = entry.filters.q
    const status = clipHistoryFilter(entry?.filters.status)
    if (status) filters.status = status
  }
  const query = new URLSearchParams(filters).toString()
  return {
    channel: currentChannel,
    path,
    href: query ? `${path}?${query}` : path,
    section: entry?.section ?? (history ? 'clips' : currentChannel),
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
      path: CLIP_LIST_PATH,
      section: entry?.section ?? 'naver_clip',
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
  if (!storage || !entry || !(isClipHistoryEntry(entry) || isMyWorkEntry(entry))) return false
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
  return entry && isClipHistoryEntry(entry) && entry.filters.restoreScroll === 'true'
    ? entry
    : undefined
}
export function completeClipHistoryReturn(ownerId: string, storage = browserStorage()): void {
  const entry = readClipHistoryReturn(ownerId, storage)
  if (!storage || !entry) return
  const filters = Object.fromEntries(
    Object.entries(entry.filters).filter(([key]) => key !== 'restoreScroll'),
  )
  store(ownerId, storage).write({ ...entry, filters })
}
/** A return to 내 글 전체 that has yet to restore its scroll, marked by `markClipHistoryReturn`. */
export function readMyWorkClipReturn(
  ownerId: string,
  storage = browserStorage(),
): ReturnContext | undefined {
  if (!storage) return undefined
  const entry = store(ownerId, storage).read()
  return entry && isMyWorkEntry(entry) && entry.filters.restoreScroll === 'true' ? entry : undefined
}
export function completeMyWorkClipReturn(ownerId: string, storage = browserStorage()): void {
  const entry = readMyWorkClipReturn(ownerId, storage)
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
  if (
    (isClipHistoryEntry(current) || isMyWorkEntry(current)) &&
    current.filters.restoreScroll === 'true'
  ) {
    const history = { ...current }
    delete history.targetId
    store(ownerId, storage).write(history)
  } else store(ownerId, storage).clear()
}
