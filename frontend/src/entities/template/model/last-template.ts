import { LAST_TEMPLATE_STORAGE_PREFIX } from '../config'

/** The template the owner last specified for a post, per account on this browser — on 홈's 템플릿
 *  dropdown or on the post guide's 템플릿 step — which the next new post starts on. '' is 없음,
 *  a real answer kept like any other; `undefined` is "never specified", where a new post starts
 *  on 없음 as it always has (TMPL-32, POST-26).
 *
 *  Browser storage only: the account has no server field for it, so it does not follow the owner
 *  to another device. A memory copy keeps it for the session where storage refuses. */
const memory = new Map<string, string>()
const listeners = new Set<() => void>()

export interface LastTemplateStorage {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
  removeItem?(key: string): void
  readonly length?: number
  key?(index: number): string | null
}

export function lastTemplateKey(ownerId: string): string {
  return LAST_TEMPLATE_STORAGE_PREFIX + encodeURIComponent(ownerId)
}

function browserStorage(): LastTemplateStorage | null {
  try {
    return window.localStorage
  } catch {
    return null
  }
}

/** Forgets every account's answer on this browser — a test's clean slate. */
export function forgetLastTemplates(storage: LastTemplateStorage | null = browserStorage()): void {
  memory.clear()
  try {
    const keys: string[] = []
    for (let index = 0; index < (storage?.length ?? 0); index++) {
      const key = storage?.key?.(index)
      if (key?.startsWith(LAST_TEMPLATE_STORAGE_PREFIX)) keys.push(key)
    }
    for (const key of keys) storage?.removeItem?.(key)
  } catch {
    /* Nothing was kept where storage refuses. */
  }
}

export function readLastTemplate(
  ownerId: string,
  storage: LastTemplateStorage | null = browserStorage(),
): string | undefined {
  if (!ownerId) return undefined
  const key = lastTemplateKey(ownerId)
  try {
    const stored = storage?.getItem(key)
    if (typeof stored === 'string') return stored
  } catch {
    /* Falls through to the session's own copy. */
  }
  return memory.get(key)
}

export function rememberLastTemplate(
  ownerId: string,
  templateId: string,
  storage: LastTemplateStorage | null = browserStorage(),
): void {
  if (!ownerId) return
  const key = lastTemplateKey(ownerId)
  memory.set(key, templateId)
  try {
    storage?.setItem(key, templateId)
  } catch {
    /* The memory copy keeps the answer for this session when browser storage rejects writes. */
  }
  for (const listener of listeners) listener()
}

/** A remembered template that is no longer in the directory is 없음, silently: the post it would
 *  have started on could not be created with it. */
export function listedLastTemplate(
  remembered: string | undefined,
  templates: readonly { id: string }[],
): string | undefined {
  if (remembered === undefined) return undefined
  return templates.some((template) => template.id === remembered) ? remembered : ''
}

/** Calls `listener` whenever an account's answer changes here or in another tab of this browser. */
export function subscribeLastTemplate(listener: () => void): () => void {
  listeners.add(listener)
  const onStorage = (event: StorageEvent) => {
    if (event.key === null || event.key.startsWith(LAST_TEMPLATE_STORAGE_PREFIX)) listener()
  }
  window.addEventListener('storage', onStorage)
  return () => {
    listeners.delete(listener)
    window.removeEventListener('storage', onStorage)
  }
}
