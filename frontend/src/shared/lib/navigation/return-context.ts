export interface ReturnContext {
  version: 1
  ownerKey: string
  path: string
  section: string
  filters: Readonly<Record<string, string>>
  scrollY: number
  targetId?: string
}
export interface NavigationStorage {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
  removeItem(key: string): void
}
export interface ReturnContextPorts {
  /** Proves that a route/target is still accessible by this owner. No default allowance. */
  isAccessible(path: string, targetId?: string): boolean
}
const STORAGE_PREFIX = 'postpilot.return.'
const MAX_RETURN_PATH_CHARS = 2048
const MAX_FILTER_VALUE_CHARS = 512

function unsafeCharacters(value: string, raw = false): boolean {
  return [...value].some((character) => {
    const code = character.charCodeAt(0)
    return code < 32 || code === 127 || code === 92 || (raw && code === 32)
  })
}
/** Check both raw and decoded input so URL normalization cannot hide unsafe separators. */
export function safeInternalPath(value: string): boolean {
  if (!value || value.length > MAX_RETURN_PATH_CHARS || unsafeCharacters(value, true)) return false
  let decoded = value
  for (let round = 0; round < 3; round += 1) {
    if (!decoded.startsWith('/') || decoded.startsWith('//') || unsafeCharacters(decoded))
      return false
    try {
      const next = decodeURIComponent(decoded)
      if (next === decoded) return true
      decoded = next
    } catch {
      return false
    }
  }
  return (
    !decoded.includes('%') &&
    decoded.startsWith('/') &&
    !decoded.startsWith('//') &&
    !unsafeCharacters(decoded)
  )
}

function record(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
export function validateReturnContext(
  value: unknown,
  ownerKey: string,
  ports: ReturnContextPorts,
): ReturnContext | undefined {
  if (
    !ownerKey ||
    !record(value) ||
    value.version !== 1 ||
    value.ownerKey !== ownerKey ||
    typeof value.path !== 'string' ||
    !safeInternalPath(value.path)
  )
    return undefined
  if (
    typeof value.section !== 'string' ||
    value.section.length > MAX_FILTER_VALUE_CHARS ||
    !record(value.filters)
  )
    return undefined
  if (typeof value.scrollY !== 'number' || !Number.isFinite(value.scrollY) || value.scrollY < 0)
    return undefined
  if (value.targetId !== undefined && (typeof value.targetId !== 'string' || !value.targetId))
    return undefined
  const filters: Record<string, string> = {}
  for (const [key, filter] of Object.entries(value.filters)) {
    if (
      ['__proto__', 'constructor', 'prototype'].includes(key) ||
      typeof filter !== 'string' ||
      filter.length > MAX_FILTER_VALUE_CHARS
    )
      return undefined
    filters[key] = filter
  }
  try {
    if (!ports.isAccessible(value.path, value.targetId)) return undefined
  } catch {
    return undefined
  }
  return {
    version: 1,
    ownerKey,
    path: value.path,
    section: value.section,
    filters,
    scrollY: value.scrollY,
    ...(typeof value.targetId === 'string' ? { targetId: value.targetId } : {}),
  }
}
/** Exception-safe, injected persistence. No account model, router or product destination. */
export function createReturnContextStore(
  ownerKey: string,
  storage: NavigationStorage,
  ports: ReturnContextPorts,
) {
  const key = `${STORAGE_PREFIX}${encodeURIComponent(ownerKey)}`
  return {
    read(): ReturnContext | undefined {
      try {
        return validateReturnContext(JSON.parse(storage.getItem(key) ?? 'null'), ownerKey, ports)
      } catch {
        return undefined
      }
    },
    write(value: ReturnContext): boolean {
      const safe = validateReturnContext(value, ownerKey, ports)
      if (!safe) return false
      try {
        storage.setItem(key, JSON.stringify(safe))
        return true
      } catch {
        return false
      }
    },
    clear(): void {
      try {
        storage.removeItem(key)
      } catch {
        /* storage may be unavailable */
      }
    },
  }
}
