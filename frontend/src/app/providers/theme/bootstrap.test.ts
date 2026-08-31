import { describe, expect, it, vi } from 'vitest'
import type { ThemeMediaQuery, ThemeStorage } from '@/shared/lib'
import { bootstrapTheme } from './bootstrap'

function targetDocument(): Document {
  const target = document.implementation.createHTMLDocument('bootstrap test')
  const themeColor = target.createElement('meta')
  themeColor.name = 'theme-color'
  themeColor.dataset.day = 'day-chrome'
  themeColor.dataset.night = 'night-chrome'
  target.head.append(themeColor)
  return target
}

function storage(value: string | null): ThemeStorage {
  return {
    getItem: vi.fn(() => value),
    setItem: vi.fn(),
    removeItem: vi.fn(),
  }
}

describe('bootstrapTheme', () => {
  it.each([
    ['light', true, 'light', 'day', 'light', 'day-chrome'],
    ['dark', false, 'dark', 'night', 'dark', 'night-chrome'],
    [null, false, 'system', 'day', 'light', 'day-chrome'],
    [null, true, 'system', 'night', 'dark', 'night-chrome'],
  ] as const)(
    'resolves stored=%j and OS dark=%s before returning the snapshot',
    (stored, prefersDark, preference, effectiveTheme, scheme, color) => {
      const target = targetDocument()
      const mediaQuery: ThemeMediaQuery = { matches: prefersDark }

      const snapshot = bootstrapTheme({
        storage: storage(stored),
        matchMedia: () => mediaQuery,
        targetDocument: target,
      })

      expect(snapshot).toEqual({ preference, effectiveTheme, prefersDark, mediaQuery })
      expect(target.documentElement.dataset.theme).toBe(effectiveTheme)
      expect(target.documentElement.style.colorScheme).toBe(scheme)
      expect(target.querySelector('meta[name="color-scheme"]')?.getAttribute('content')).toBe(
        scheme,
      )
      expect(target.querySelector('meta[name="theme-color"]')?.getAttribute('content')).toBe(color)
    },
  )

  it('falls back to System while preserving a readable OS preference when storage fails', () => {
    const target = targetDocument()
    const denied: ThemeStorage = {
      getItem: () => {
        throw new DOMException('denied')
      },
      setItem: vi.fn(),
      removeItem: vi.fn(),
    }

    const snapshot = bootstrapTheme({
      storage: denied,
      matchMedia: () => ({ matches: true }),
      targetDocument: target,
    })

    expect(snapshot.preference).toBe('system')
    expect(snapshot.effectiveTheme).toBe('night')
    expect(target.documentElement.dataset.theme).toBe('night')
  })

  it.each(['system', 'invalid', ' dark '] as const)(
    'treats malformed stored value %j as System',
    (stored) => {
      const snapshot = bootstrapTheme({
        storage: storage(stored),
        matchMedia: () => ({ matches: false }),
        targetDocument: targetDocument(),
      })

      expect(snapshot.preference).toBe('system')
      expect(snapshot.effectiveTheme).toBe('day')
    },
  )

  it('survives unavailable browser capabilities and document', () => {
    expect(bootstrapTheme({ storage: null, matchMedia: null, targetDocument: null })).toEqual({
      preference: 'system',
      effectiveTheme: 'day',
      prefersDark: false,
    })
  })
})
