import {
  createReturnContextStore,
  type ReturnContext,
  type NavigationStorage,
} from '@/shared/lib/navigation'

import { CLIP_CHANNELS, DEFAULT_CLIP_CHANNEL, type ClipChannel } from '../config/channel'

export interface ClipEntry {
  path: '/naver-clip' | '/insta-clip'
  section: string
  filters: Readonly<Record<string, string>>
  scrollY: number
  targetId?: string
}
export function clipChannelPath(channel: ClipChannel): '/naver-clip' | '/insta-clip' {
  return channel === 'instagram_clip' ? '/insta-clip' : '/naver-clip'
}
export interface ClipReturnDestination {
  channel: ClipChannel
  path: ClipEntry['path']
  href: string
  section: string
  scrollY: number
}
/** Each clip channel opens on its own history (CLIP-41), so history and creation share the
 *  channel's address and the section tells them apart. A `/clips` entry stored before that still
 *  names the history. */
export function isClipHistoryEntry(entry: Pick<ReturnContext, 'path' | 'section'>): boolean {
  return entry.section === 'clips' || entry.path === '/clips'
}
function storedChannel(entry: ReturnContext | undefined): ClipChannel {
  return CLIP_CHANNELS.includes(entry?.filters.channel as ClipChannel)
    ? (entry!.filters.channel as ClipChannel)
    : entry?.path === '/insta-clip'
      ? 'instagram_clip'
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
      isAccessible: (path) =>
        ['/naver-clip', '/insta-clip', '/clips', '/', '/library'].includes(path),
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
  if (entry.path === '/' || entry.path === '/library')
    return {
      ...entry,
      path: clipChannelPath(
        entry.filters.channel === 'instagram_clip' ? 'instagram_clip' : DEFAULT_CLIP_CHANNEL,
      ),
    }
  return entry.path === '/clips'
    ? { ...entry, path: clipChannelPath(storedChannel(entry)), section: 'clips' }
    : entry
}
export function clipReturnDestination(
  ownerId: string,
  clipId?: string,
  storage = browserStorage(),
  channel?: ClipChannel,
): ClipReturnDestination {
  const entry = readClipReturnContext(ownerId, clipId, storage)
  const currentChannel = channel ?? storedChannel(entry)
  // The channel's address carries the channel; a history entry adds its own narrowing. A clip
  // moved to the other channel returns to that channel's history (CLIP-209).
  const path = clipChannelPath(currentChannel)
  const history = entry ? isClipHistoryEntry(entry) : !!clipId
  const filters: Record<string, string> = {}
  if (history) {
    if (entry?.filters.q) filters.q = entry.filters.q
    if (entry?.filters.status && ['draft', 'refining', 'finished'].includes(entry.filters.status))
      filters.status = entry.filters.status
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
      path: clipChannelPath(
        entry?.filters.channel === 'instagram_clip' || entry?.path === '/insta-clip'
          ? 'instagram_clip'
          : DEFAULT_CLIP_CHANNEL,
      ),
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
  if (!storage || !entry || !isClipHistoryEntry(entry)) return false
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
/** Confirmed deletion removes the record's origin while retaining a pending directory scroll. */
export function forgetClipEntry(ownerId: string, clipId: string, storage = browserStorage()): void {
  if (!storage) return
  store(ownerId, storage, clipId).clear()
  const current = store(ownerId, storage).read()
  if (current?.targetId !== clipId) return
  if (isClipHistoryEntry(current) && current.filters.restoreScroll === 'true') {
    const history = { ...current }
    delete history.targetId
    store(ownerId, storage).write(history)
  } else store(ownerId, storage).clear()
}
