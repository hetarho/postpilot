import { DEFAULT_THEME_PREFERENCE, THEME_PREFERENCE_STORAGE_KEY } from '@/shared/config'
import {
  parseStoredThemePreference,
  resolveEffectiveTheme,
  type EffectiveTheme,
  type ThemePreference,
} from './contract'

export const PREFERS_DARK_MEDIA_QUERY = '(prefers-color-scheme: dark)' as const

export interface ThemeStorage {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
  removeItem(key: string): void
}

export type ThemeStorageReader = Pick<ThemeStorage, 'getItem'>
export type ThemeStorageWriter = Pick<ThemeStorage, 'setItem' | 'removeItem'>

export interface ThemeMediaQuery {
  readonly matches: boolean
  addEventListener?: (type: 'change', listener: (event: MediaQueryListEvent) => void) => void
  removeEventListener?: (type: 'change', listener: (event: MediaQueryListEvent) => void) => void
}

export type ThemeMatchMedia = (query: string) => ThemeMediaQuery

/** Pass null to model an unavailable browser capability; omission uses the real browser. */
export interface ThemeBrowserPorts {
  storage?: ThemeStorage | null
  matchMedia?: ThemeMatchMedia | null
}

export interface BrowserThemeSnapshot {
  preference: ThemePreference
  effectiveTheme: EffectiveTheme
  prefersDark: boolean
  mediaQuery?: ThemeMediaQuery
}

export function browserThemeStorage(): ThemeStorage | undefined {
  try {
    return typeof globalThis.localStorage === 'undefined' ? undefined : globalThis.localStorage
  } catch {
    return undefined
  }
}

export function readThemePreference(
  storage: ThemeStorageReader | null | undefined = browserThemeStorage(),
): ThemePreference {
  try {
    return (
      parseStoredThemePreference(storage?.getItem(THEME_PREFERENCE_STORAGE_KEY)) ??
      DEFAULT_THEME_PREFERENCE
    )
  } catch {
    return DEFAULT_THEME_PREFERENCE
  }
}

/** Returns false only when persistence is unavailable; the caller can keep in-memory state. */
export function writeThemePreference(
  preference: ThemePreference,
  storage: ThemeStorageWriter | null | undefined = browserThemeStorage(),
): boolean {
  if (!storage) return false
  try {
    if (preference === DEFAULT_THEME_PREFERENCE) {
      storage.removeItem(THEME_PREFERENCE_STORAGE_KEY)
    } else {
      storage.setItem(THEME_PREFERENCE_STORAGE_KEY, preference)
    }
    return true
  } catch {
    return false
  }
}

export function browserThemeMatchMedia(): ThemeMatchMedia | undefined {
  try {
    const matchMedia = globalThis.matchMedia
    return typeof matchMedia === 'function' ? matchMedia.bind(globalThis) : undefined
  } catch {
    return undefined
  }
}

export function browserThemeMediaQuery(
  matchMedia: ThemeMatchMedia | null | undefined = browserThemeMatchMedia(),
): ThemeMediaQuery | undefined {
  if (!matchMedia) return undefined
  try {
    return matchMedia(PREFERS_DARK_MEDIA_QUERY)
  } catch {
    return undefined
  }
}

export function readPrefersDark(
  mediaQuery: ThemeMediaQuery | null | undefined = browserThemeMediaQuery(),
): boolean {
  try {
    return mediaQuery?.matches === true
  } catch {
    return false
  }
}

export function resolveBrowserTheme(ports: ThemeBrowserPorts = {}): BrowserThemeSnapshot {
  const storage = ports.storage === null ? null : (ports.storage ?? browserThemeStorage() ?? null)
  const matchMedia =
    ports.matchMedia === null ? null : (ports.matchMedia ?? browserThemeMatchMedia() ?? null)
  const mediaQuery = browserThemeMediaQuery(matchMedia)
  const preference = readThemePreference(storage)
  const prefersDark = readPrefersDark(mediaQuery ?? null)

  return {
    preference,
    effectiveTheme: resolveEffectiveTheme(preference, prefersDark),
    prefersDark,
    ...(mediaQuery ? { mediaQuery } : {}),
  }
}
