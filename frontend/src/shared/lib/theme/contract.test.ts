import { afterEach, describe, expect, expectTypeOf, it, vi } from 'vitest'
import { DEFAULT_THEME_PREFERENCE, THEME_PREFERENCE_STORAGE_KEY } from '@/shared/config'
import {
  PREFERS_DARK_MEDIA_QUERY,
  browserThemeMatchMedia,
  browserThemeMediaQuery,
  browserThemeStorage,
  isEffectiveTheme,
  isThemePreference,
  parseStoredThemePreference,
  readPrefersDark,
  readThemePreference,
  resolveBrowserTheme,
  resolveEffectiveTheme,
  writeThemePreference,
  type EffectiveTheme,
  type ThemeMediaQuery,
  type ThemePreference,
} from '.'

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('theme contract', () => {
  it('keeps the fixed config values as literal types', () => {
    expectTypeOf(DEFAULT_THEME_PREFERENCE).toEqualTypeOf<'system'>()
    expectTypeOf(THEME_PREFERENCE_STORAGE_KEY).toEqualTypeOf<'postpilot.theme'>()
  })

  it.each([
    ['system', true],
    ['light', true],
    ['dark', true],
    ['', false],
    ['LIGHT', false],
    ['night', false],
    [null, false],
    [undefined, false],
    [1, false],
  ])('checks preference %j strictly', (value, expected) => {
    expect(isThemePreference(value)).toBe(expected)
  })

  it.each([
    ['day', true],
    ['night', true],
    ['light', false],
    ['dark', false],
    ['', false],
    [null, false],
  ])('checks effective theme %j strictly', (value, expected) => {
    expect(isEffectiveTheme(value)).toBe(expected)
  })

  it.each([
    ['light', 'light'],
    ['dark', 'dark'],
    ['system', undefined],
    [' light ', undefined],
    ['LIGHT', undefined],
    ['', undefined],
    [null, undefined],
    [undefined, undefined],
    [{}, undefined],
  ])('parses stored override %j without normalization', (value, expected) => {
    expect(parseStoredThemePreference(value)).toBe(expected)
  })

  it.each<[ThemePreference, boolean, EffectiveTheme]>([
    ['system', false, 'day'],
    ['system', true, 'night'],
    ['light', false, 'day'],
    ['light', true, 'day'],
    ['dark', false, 'night'],
    ['dark', true, 'night'],
  ])('resolves %s with prefersDark=%s to %s', (preference, prefersDark, expected) => {
    expect(resolveEffectiveTheme(preference, prefersDark)).toBe(expected)
  })
})

describe('theme storage adapter', () => {
  it.each([
    ['light', 'light'],
    ['dark', 'dark'],
    [null, 'system'],
    ['system', 'system'],
    ['en', 'system'],
    [' dark ', 'system'],
  ] as const)('reads %j as %s', (stored, expected) => {
    const getItem = vi.fn((key: string) => {
      expect(key).toBe(THEME_PREFERENCE_STORAGE_KEY)
      return stored
    })

    expect(readThemePreference({ getItem })).toBe(expected)
    expect(getItem).toHaveBeenCalledOnce()
  })

  it('falls back to System when storage is missing or throws', () => {
    expect(readThemePreference(null)).toBe(DEFAULT_THEME_PREFERENCE)
    expect(
      readThemePreference({
        getItem: () => {
          throw new DOMException('denied')
        },
      }),
    ).toBe(DEFAULT_THEME_PREFERENCE)
  })

  it.each(['light', 'dark'] as const)('writes the exact %s override', (preference) => {
    const storage = {
      getItem: vi.fn(),
      setItem: vi.fn(),
      removeItem: vi.fn(),
    }

    expect(writeThemePreference(preference, storage)).toBe(true)
    expect(storage.setItem).toHaveBeenCalledWith(THEME_PREFERENCE_STORAGE_KEY, preference)
    expect(storage.removeItem).not.toHaveBeenCalled()
  })

  it('represents System by removing the explicit override', () => {
    const storage = {
      getItem: vi.fn(),
      setItem: vi.fn(),
      removeItem: vi.fn(),
    }

    expect(writeThemePreference('system', storage)).toBe(true)
    expect(storage.removeItem).toHaveBeenCalledWith(THEME_PREFERENCE_STORAGE_KEY)
    expect(storage.setItem).not.toHaveBeenCalled()
  })

  it.each(['light', 'system'] as const)(
    'reports but never throws when persisting %s fails',
    (preference) => {
      const denied = () => {
        throw new DOMException('denied')
      }
      const storage = { getItem: vi.fn(), setItem: denied, removeItem: denied }

      expect(writeThemePreference(preference, storage)).toBe(false)
    },
  )

  it('reports unavailable storage without throwing', () => {
    expect(writeThemePreference('dark', null)).toBe(false)
  })
})

describe('theme browser adapter', () => {
  it('queries the one colour-scheme media contract and preserves the query object', () => {
    const mediaQuery: ThemeMediaQuery = { matches: true }
    const matchMedia = vi.fn(() => mediaQuery)

    expect(browserThemeMediaQuery(matchMedia)).toBe(mediaQuery)
    expect(matchMedia).toHaveBeenCalledWith(PREFERS_DARK_MEDIA_QUERY)
    expect(readPrefersDark(mediaQuery)).toBe(true)
  })

  it('treats missing and throwing media access as a light OS preference', () => {
    expect(browserThemeMediaQuery(null)).toBeUndefined()
    expect(
      browserThemeMediaQuery(() => {
        throw new DOMException('unavailable')
      }),
    ).toBeUndefined()
    expect(readPrefersDark(null)).toBe(false)
    expect(
      readPrefersDark({
        get matches(): boolean {
          throw new DOMException('unavailable')
        },
      }),
    ).toBe(false)
  })

  it.each([
    ['light', true, 'light', 'day'],
    ['dark', false, 'dark', 'night'],
    [null, true, 'system', 'night'],
    ['system', false, 'system', 'day'],
    ['unknown', true, 'system', 'night'],
  ] as const)(
    'resolves stored=%j and prefersDark=%s to %s/%s',
    (stored, prefersDark, preference, effectiveTheme) => {
      const mediaQuery: ThemeMediaQuery = { matches: prefersDark }
      const snapshot = resolveBrowserTheme({
        storage: {
          getItem: () => stored,
          setItem: vi.fn(),
          removeItem: vi.fn(),
        },
        matchMedia: () => mediaQuery,
      })

      expect(snapshot).toEqual({ preference, effectiveTheme, prefersDark, mediaQuery })
    },
  )

  it('falls back safely when both browser capabilities are explicitly unavailable', () => {
    expect(resolveBrowserTheme({ storage: null, matchMedia: null })).toEqual({
      preference: 'system',
      effectiveTheme: 'day',
      prefersDark: false,
    })
  })

  it('falls back to System while retaining the readable OS preference when storage throws', () => {
    const snapshot = resolveBrowserTheme({
      storage: {
        getItem: () => {
          throw new DOMException('denied')
        },
        setItem: vi.fn(),
        removeItem: vi.fn(),
      },
      matchMedia: () => ({ matches: true }),
    })

    expect(snapshot.preference).toBe('system')
    expect(snapshot.effectiveTheme).toBe('night')
  })

  it('uses safe real-browser adapters and survives SSR-like missing globals', () => {
    vi.stubGlobal('localStorage', undefined)
    vi.stubGlobal('matchMedia', undefined)

    expect(browserThemeStorage()).toBeUndefined()
    expect(browserThemeMatchMedia()).toBeUndefined()
    expect(resolveBrowserTheme()).toEqual({
      preference: 'system',
      effectiveTheme: 'day',
      prefersDark: false,
    })
  })
})
