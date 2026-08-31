import type { ThemeRuntimePorts, ThemeStorageEventTarget } from '@/app/providers/theme'
import { THEME_PREFERENCE_STORAGE_KEY } from '@/shared/config'
import { PREFERS_DARK_MEDIA_QUERY, type ThemeMediaQuery, type ThemeStorage } from '@/shared/lib'

export class MemoryThemeStorage implements ThemeStorage {
  private readonly values = new Map<string, string>()

  constructor(storedPreference: string | null = null) {
    if (storedPreference !== null) {
      this.values.set(THEME_PREFERENCE_STORAGE_KEY, storedPreference)
    }
  }

  getItem(key: string): string | null {
    return this.values.get(key) ?? null
  }

  setItem(key: string, value: string): void {
    this.values.set(key, value)
  }

  removeItem(key: string): void {
    this.values.delete(key)
  }
}

export class ControllableThemeMediaQuery implements ThemeMediaQuery {
  private readonly listeners = new Set<(event: MediaQueryListEvent) => void>()
  private currentMatches: boolean

  constructor(prefersDark = false) {
    this.currentMatches = prefersDark
  }

  get matches(): boolean {
    return this.currentMatches
  }

  addEventListener(_type: 'change', listener: (event: MediaQueryListEvent) => void): void {
    this.listeners.add(listener)
  }

  removeEventListener(_type: 'change', listener: (event: MediaQueryListEvent) => void): void {
    this.listeners.delete(listener)
  }

  change(prefersDark: boolean): void {
    if (prefersDark === this.currentMatches) return
    this.currentMatches = prefersDark
    const event = {
      matches: prefersDark,
      media: PREFERS_DARK_MEDIA_QUERY,
    } as MediaQueryListEvent
    for (const listener of this.listeners) listener(event)
  }

  listenerCount(): number {
    return this.listeners.size
  }
}

export class ControllableThemeStorageEvents implements ThemeStorageEventTarget {
  private readonly listeners = new Set<(event: StorageEvent) => void>()

  addEventListener(_type: 'storage', listener: (event: StorageEvent) => void): void {
    this.listeners.add(listener)
  }

  removeEventListener(_type: 'storage', listener: (event: StorageEvent) => void): void {
    this.listeners.delete(listener)
  }

  emit(newValue: string | null, key: string | null = THEME_PREFERENCE_STORAGE_KEY): void {
    const event = { key, newValue, storageArea: null } as StorageEvent
    for (const listener of this.listeners) listener(event)
  }

  listenerCount(): number {
    return this.listeners.size
  }
}

export interface ThemeTestEnvironment {
  storage: MemoryThemeStorage
  mediaQuery: ControllableThemeMediaQuery
  storageEvents: ControllableThemeStorageEvents
  ports: ThemeRuntimePorts
  changeStorageFromAnotherTab(value: string | null): void
}

export function createThemeTestEnvironment({
  storedPreference = null,
  prefersDark = false,
}: {
  storedPreference?: string | null
  prefersDark?: boolean
} = {}): ThemeTestEnvironment {
  const storage = new MemoryThemeStorage(storedPreference)
  const mediaQuery = new ControllableThemeMediaQuery(prefersDark)
  const storageEvents = new ControllableThemeStorageEvents()

  return {
    storage,
    mediaQuery,
    storageEvents,
    ports: { storage, mediaQuery, storageEvents },
    changeStorageFromAnotherTab(value) {
      if (value === null) storage.removeItem(THEME_PREFERENCE_STORAGE_KEY)
      else storage.setItem(THEME_PREFERENCE_STORAGE_KEY, value)
      storageEvents.emit(value)
    },
  }
}
