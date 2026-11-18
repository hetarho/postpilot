import { useSyncExternalStore } from 'react'

/** What 홈's centre starts: a post (글쓰기) or a video (영상 만들기). The owner switches it from the
 *  centre's own dropdown, and 홈 opens on the last one chosen, per account on this browser.
 *
 *  Browser storage only, as the template the next post starts on is (`lastTemplate`): the account
 *  has no server field for it. A memory copy keeps it for the session where storage refuses. */
export const CREATION_MODES = ['post', 'video'] as const
export type CreationMode = (typeof CREATION_MODES)[number]

export const CREATION_MODE_STORAGE_PREFIX = 'postpilot.home-creation.v1.'

const memory = new Map<string, CreationMode>()
const listeners = new Set<() => void>()

export interface CreationModeStorage {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
  removeItem?(key: string): void
  readonly length?: number
  key?(index: number): string | null
}

export function creationModeKey(ownerId: string): string {
  return CREATION_MODE_STORAGE_PREFIX + encodeURIComponent(ownerId)
}

function browserStorage(): CreationModeStorage | null {
  try {
    return window.localStorage
  } catch {
    return null
  }
}

function isCreationMode(value: unknown): value is CreationMode {
  return (CREATION_MODES as readonly unknown[]).includes(value)
}

/** Forgets every account's answer on this browser — a test's clean slate. */
export function forgetCreationModes(storage: CreationModeStorage | null = browserStorage()): void {
  memory.clear()
  try {
    const keys: string[] = []
    for (let index = 0; index < (storage?.length ?? 0); index++) {
      const key = storage?.key?.(index)
      if (key?.startsWith(CREATION_MODE_STORAGE_PREFIX)) keys.push(key)
    }
    for (const key of keys) storage?.removeItem?.(key)
  } catch {
    /* Nothing was kept where storage refuses. */
  }
}

/** The account's last choice; 글쓰기 until one has been made, or when what is stored is not one. */
export function readCreationMode(
  ownerId: string,
  storage: CreationModeStorage | null = browserStorage(),
): CreationMode {
  if (!ownerId) return 'post'
  const key = creationModeKey(ownerId)
  try {
    const stored = storage?.getItem(key)
    if (isCreationMode(stored)) return stored
  } catch {
    /* Falls through to the session's own copy. */
  }
  return memory.get(key) ?? 'post'
}

export function rememberCreationMode(
  ownerId: string,
  mode: CreationMode,
  storage: CreationModeStorage | null = browserStorage(),
): void {
  if (!ownerId) return
  const key = creationModeKey(ownerId)
  memory.set(key, mode)
  try {
    storage?.setItem(key, mode)
  } catch {
    /* The memory copy keeps the answer for this session when browser storage rejects writes. */
  }
  for (const listener of listeners) listener()
}

function subscribeCreationMode(listener: () => void): () => void {
  listeners.add(listener)
  const onStorage = (event: StorageEvent) => {
    if (event.key === null || event.key.startsWith(CREATION_MODE_STORAGE_PREFIX)) listener()
  }
  window.addEventListener('storage', onStorage)
  return () => {
    listeners.delete(listener)
    window.removeEventListener('storage', onStorage)
  }
}

/** The account's choice, kept current as it changes here or in another tab of this browser. */
export function useCreationMode(ownerId: string): CreationMode {
  return useSyncExternalStore(
    subscribeCreationMode,
    () => readCreationMode(ownerId),
    () => 'post',
  )
}
