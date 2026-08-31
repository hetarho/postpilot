import { StrictMode, type ReactNode } from 'react'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useThemeController } from '@/features/change-theme'
import { THEME_PREFERENCE_STORAGE_KEY } from '@/shared/config'
import {
  resolveEffectiveTheme,
  type BrowserThemeSnapshot,
  type ThemeMediaQuery,
  type ThemePreference,
  type ThemeStorage,
} from '@/shared/lib'
import {
  ThemeProvider,
  type ThemeRuntimePorts,
  type ThemeStorageEventTarget,
} from './ThemeProvider'

class FakeMediaQuery implements ThemeMediaQuery {
  matches: boolean
  readonly addEventListener = vi.fn(
    (_type: 'change', listener: (event: MediaQueryListEvent) => void) => {
      this.listeners.add(listener)
    },
  )
  readonly removeEventListener = vi.fn(
    (_type: 'change', listener: (event: MediaQueryListEvent) => void) => {
      this.listeners.delete(listener)
    },
  )
  private readonly listeners = new Set<(event: MediaQueryListEvent) => void>()

  constructor(matches: boolean) {
    this.matches = matches
  }

  change(matches: boolean): void {
    this.matches = matches
    const event = { matches, media: '(prefers-color-scheme: dark)' } as MediaQueryListEvent
    for (const listener of this.listeners) listener(event)
  }

  listenerCount(): number {
    return this.listeners.size
  }
}

class FakeStorageEvents implements ThemeStorageEventTarget {
  readonly addEventListener = vi.fn((_type: 'storage', listener: (event: StorageEvent) => void) => {
    this.listeners.add(listener)
  })
  readonly removeEventListener = vi.fn(
    (_type: 'storage', listener: (event: StorageEvent) => void) => {
      this.listeners.delete(listener)
    },
  )
  private readonly listeners = new Set<(event: StorageEvent) => void>()

  emit(key: string | null, newValue: string | null): void {
    const event = { key, newValue, storageArea: null } as StorageEvent
    for (const listener of this.listeners) listener(event)
  }

  listenerCount(): number {
    return this.listeners.size
  }
}

function fakeStorage(value: string | null = null) {
  return {
    getItem: vi.fn<(key: string) => string | null>(() => value),
    setItem: vi.fn<(key: string, value: string) => void>(),
    removeItem: vi.fn<(key: string) => void>(),
  }
}

function snapshot(
  preference: ThemePreference,
  prefersDark: boolean,
  mediaQuery?: ThemeMediaQuery,
): BrowserThemeSnapshot {
  return {
    preference,
    prefersDark,
    effectiveTheme: resolveEffectiveTheme(preference, prefersDark),
    ...(mediaQuery ? { mediaQuery } : {}),
  }
}

function Probe() {
  const controller = useThemeController()
  return (
    <div>
      <output data-testid="preference">{controller.preference}</output>
      <output data-testid="effective-theme">{controller.effectiveTheme}</output>
      {(['system', 'light', 'dark'] as const).map((preference) => (
        <button key={preference} type="button" onClick={() => controller.setPreference(preference)}>
          {preference}
        </button>
      ))}
    </div>
  )
}

function Runtime({
  initialSnapshot,
  ports,
  children = <Probe />,
}: {
  initialSnapshot: BrowserThemeSnapshot
  ports: ThemeRuntimePorts
  children?: ReactNode
}) {
  return (
    <ThemeProvider initialSnapshot={initialSnapshot} ports={ports}>
      {children}
    </ThemeProvider>
  )
}

beforeEach(() => {
  document.documentElement.removeAttribute('data-theme')
  document.documentElement.removeAttribute('style')
  document.head.querySelector('meta[name="theme-color"]')?.remove()
  document.head.querySelector('meta[name="color-scheme"]')?.remove()
  const themeColor = document.createElement('meta')
  themeColor.name = 'theme-color'
  themeColor.dataset.day = 'day-chrome'
  themeColor.dataset.night = 'night-chrome'
  document.head.append(themeColor)
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('ThemeProvider', () => {
  it('starts from the bootstrap snapshot without rewriting persistence', () => {
    const storage = fakeStorage()
    const mediaQuery = new FakeMediaQuery(true)
    const storageEvents = new FakeStorageEvents()

    render(
      <Runtime
        initialSnapshot={snapshot('system', true, mediaQuery)}
        ports={{ storage, mediaQuery, storageEvents }}
      />,
    )

    expect(screen.getByTestId('preference')).toHaveTextContent('system')
    expect(screen.getByTestId('effective-theme')).toHaveTextContent('night')
    expect(document.documentElement).toHaveAttribute('data-theme', 'night')
    expect(document.head.querySelector('meta[name="theme-color"]')).toHaveAttribute(
      'content',
      'night-chrome',
    )
    expect(mediaQuery.listenerCount()).toBe(1)
    expect(storage.setItem).not.toHaveBeenCalled()
    expect(storage.removeItem).not.toHaveBeenCalled()
  })

  it('applies and persists explicit choices, then removes the override for System', () => {
    const storage = fakeStorage()
    const mediaQuery = new FakeMediaQuery(true)

    render(
      <Runtime
        initialSnapshot={snapshot('system', true, mediaQuery)}
        ports={{ storage, mediaQuery, storageEvents: new FakeStorageEvents() }}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: 'light' }))
    expect(screen.getByTestId('preference')).toHaveTextContent('light')
    expect(screen.getByTestId('effective-theme')).toHaveTextContent('day')
    expect(document.documentElement).toHaveAttribute('data-theme', 'day')
    expect(document.documentElement.style.colorScheme).toBe('light')
    expect(document.head.querySelector('meta[name="theme-color"]')).toHaveAttribute(
      'content',
      'day-chrome',
    )
    expect(storage.setItem).toHaveBeenLastCalledWith(THEME_PREFERENCE_STORAGE_KEY, 'light')
    expect(mediaQuery.listenerCount()).toBe(0)

    mediaQuery.change(false)
    expect(screen.getByTestId('effective-theme')).toHaveTextContent('day')

    fireEvent.click(screen.getByRole('button', { name: 'system' }))
    expect(screen.getByTestId('preference')).toHaveTextContent('system')
    expect(screen.getByTestId('effective-theme')).toHaveTextContent('day')
    expect(storage.removeItem).toHaveBeenLastCalledWith(THEME_PREFERENCE_STORAGE_KEY)
    expect(mediaQuery.listenerCount()).toBe(1)
  })

  it('follows OS changes only while System is active', () => {
    const mediaQuery = new FakeMediaQuery(false)
    render(
      <Runtime
        initialSnapshot={snapshot('system', false, mediaQuery)}
        ports={{ storage: fakeStorage(), mediaQuery, storageEvents: new FakeStorageEvents() }}
      />,
    )

    act(() => mediaQuery.change(true))
    expect(screen.getByTestId('effective-theme')).toHaveTextContent('night')
    expect(document.documentElement).toHaveAttribute('data-theme', 'night')

    fireEvent.click(screen.getByRole('button', { name: 'dark' }))
    expect(mediaQuery.listenerCount()).toBe(0)
    act(() => mediaQuery.change(false))
    expect(screen.getByTestId('effective-theme')).toHaveTextContent('night')
  })

  it('reconciles an OS change missed between bootstrap and listener attachment', () => {
    const storage = fakeStorage()
    const mediaQuery = new FakeMediaQuery(false)
    const initialSnapshot = snapshot('system', false, mediaQuery)

    mediaQuery.change(true)
    render(
      <Runtime
        initialSnapshot={initialSnapshot}
        ports={{ storage, mediaQuery, storageEvents: new FakeStorageEvents() }}
      />,
    )

    expect(screen.getByTestId('preference')).toHaveTextContent('system')
    expect(screen.getByTestId('effective-theme')).toHaveTextContent('night')
    expect(document.documentElement).toHaveAttribute('data-theme', 'night')
    expect(mediaQuery.listenerCount()).toBe(1)
    expect(storage.setItem).not.toHaveBeenCalled()
    expect(storage.removeItem).not.toHaveBeenCalled()
  })

  it('reconciles a storage change missed between bootstrap and listener attachment', () => {
    const storage = fakeStorage('dark')
    const mediaQuery = new FakeMediaQuery(false)
    const storageEvents = new FakeStorageEvents()

    render(
      <Runtime
        initialSnapshot={snapshot('system', false, mediaQuery)}
        ports={{ storage, mediaQuery, storageEvents }}
      />,
    )

    expect(screen.getByTestId('preference')).toHaveTextContent('dark')
    expect(screen.getByTestId('effective-theme')).toHaveTextContent('night')
    expect(document.documentElement).toHaveAttribute('data-theme', 'night')
    expect(storageEvents.listenerCount()).toBe(1)
    expect(mediaQuery.listenerCount()).toBe(0)
    expect(storage.setItem).not.toHaveBeenCalled()
    expect(storage.removeItem).not.toHaveBeenCalled()
  })

  it('accepts explicit, removed, and malformed cross-tab values without writing them back', () => {
    const storage = fakeStorage()
    const mediaQuery = new FakeMediaQuery(true)
    const storageEvents = new FakeStorageEvents()
    render(
      <Runtime
        initialSnapshot={snapshot('dark', true, mediaQuery)}
        ports={{ storage, mediaQuery, storageEvents }}
      />,
    )

    act(() => storageEvents.emit(THEME_PREFERENCE_STORAGE_KEY, 'light'))
    expect(screen.getByTestId('preference')).toHaveTextContent('light')
    expect(screen.getByTestId('effective-theme')).toHaveTextContent('day')

    act(() => storageEvents.emit(THEME_PREFERENCE_STORAGE_KEY, null))
    expect(screen.getByTestId('preference')).toHaveTextContent('system')
    expect(screen.getByTestId('effective-theme')).toHaveTextContent('night')

    act(() => storageEvents.emit(THEME_PREFERENCE_STORAGE_KEY, 'sepia'))
    expect(screen.getByTestId('preference')).toHaveTextContent('system')
    expect(storage.setItem).not.toHaveBeenCalled()
    expect(storage.removeItem).not.toHaveBeenCalled()
  })

  it('keeps the in-memory choice when persistence throws', () => {
    const denied = () => {
      throw new DOMException('denied')
    }
    const storage: ThemeStorage = { getItem: denied, setItem: denied, removeItem: denied }

    render(
      <Runtime
        initialSnapshot={snapshot('system', false)}
        ports={{ storage, mediaQuery: null, storageEvents: new FakeStorageEvents() }}
      />,
    )

    expect(() => fireEvent.click(screen.getByRole('button', { name: 'dark' }))).not.toThrow()
    expect(screen.getByTestId('preference')).toHaveTextContent('dark')
    expect(screen.getByTestId('effective-theme')).toHaveTextContent('night')
    expect(document.documentElement).toHaveAttribute('data-theme', 'night')
  })

  it('preserves explicit null ports without reacquiring browser globals', () => {
    const browserStorage = {
      getItem: vi.fn<(key: string) => string | null>(() => {
        throw new DOMException('unexpected storage read')
      }),
      setItem: vi.fn<(key: string, value: string) => void>(() => {
        throw new DOMException('unexpected storage write')
      }),
      removeItem: vi.fn<(key: string) => void>(() => {
        throw new DOMException('unexpected storage removal')
      }),
    }
    const matchMedia = vi.fn(() => new FakeMediaQuery(true))
    vi.stubGlobal('localStorage', browserStorage)
    vi.stubGlobal('matchMedia', matchMedia)

    render(
      <Runtime
        initialSnapshot={snapshot('system', false)}
        ports={{ storage: null, mediaQuery: null, storageEvents: null, targetDocument: null }}
      />,
    )
    fireEvent.click(screen.getByRole('button', { name: 'dark' }))

    expect(screen.getByTestId('preference')).toHaveTextContent('dark')
    expect(screen.getByTestId('effective-theme')).toHaveTextContent('night')
    expect(document.documentElement).not.toHaveAttribute('data-theme')

    fireEvent.click(screen.getByRole('button', { name: 'system' }))
    expect(screen.getByTestId('preference')).toHaveTextContent('system')
    expect(screen.getByTestId('effective-theme')).toHaveTextContent('day')
    expect(browserStorage.getItem).not.toHaveBeenCalled()
    expect(browserStorage.setItem).not.toHaveBeenCalled()
    expect(browserStorage.removeItem).not.toHaveBeenCalled()
    expect(matchMedia).not.toHaveBeenCalled()
  })

  it('cleans and reattaches media/storage listeners under StrictMode and mode changes', () => {
    const mediaQuery = new FakeMediaQuery(false)
    const storageEvents = new FakeStorageEvents()
    const view = render(
      <StrictMode>
        <Runtime
          initialSnapshot={snapshot('system', false, mediaQuery)}
          ports={{ storage: fakeStorage(), mediaQuery, storageEvents }}
        />
      </StrictMode>,
    )

    expect(mediaQuery.listenerCount()).toBe(1)
    expect(storageEvents.listenerCount()).toBe(1)
    expect(mediaQuery.addEventListener.mock.calls.length).toBe(
      mediaQuery.removeEventListener.mock.calls.length + 1,
    )
    expect(storageEvents.addEventListener.mock.calls.length).toBe(
      storageEvents.removeEventListener.mock.calls.length + 1,
    )

    fireEvent.click(screen.getByRole('button', { name: 'light' }))
    expect(mediaQuery.listenerCount()).toBe(0)
    expect(storageEvents.listenerCount()).toBe(1)

    view.unmount()
    expect(mediaQuery.listenerCount()).toBe(0)
    expect(storageEvents.listenerCount()).toBe(0)
    expect(mediaQuery.addEventListener.mock.calls.length).toBe(
      mediaQuery.removeEventListener.mock.calls.length,
    )
    expect(storageEvents.addEventListener.mock.calls.length).toBe(
      storageEvents.removeEventListener.mock.calls.length,
    )
  })
})
