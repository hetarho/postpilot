import { describe, expect, it } from 'vitest'
import { applyDocumentTheme } from './document-theme'

function emptyDocument(): Document {
  return document.implementation.createHTMLDocument('theme test')
}

function themeDocument(): Document {
  const target = emptyDocument()
  const themeColor = target.createElement('meta')
  themeColor.name = 'theme-color'
  themeColor.dataset.day = 'day-chrome'
  themeColor.dataset.night = 'night-chrome'
  target.head.append(themeColor)
  return target
}

describe('applyDocumentTheme', () => {
  it.each([
    ['day', 'light', 'day-chrome'],
    ['night', 'dark', 'night-chrome'],
  ] as const)(
    'keeps %s DOM, native controls, and browser chrome in parity',
    (theme, scheme, color) => {
      const target = themeDocument()

      applyDocumentTheme(theme, target)

      expect(target.documentElement.dataset.theme).toBe(theme)
      expect(target.documentElement.style.colorScheme).toBe(scheme)
      expect(target.querySelector('meta[name="color-scheme"]')?.getAttribute('content')).toBe(
        scheme,
      )
      expect(target.querySelector('meta[name="theme-color"]')?.getAttribute('content')).toBe(color)
    },
  )

  it('updates existing metadata without creating parallel sources of truth', () => {
    const target = themeDocument()

    applyDocumentTheme('night', target)
    applyDocumentTheme('day', target)

    expect(target.querySelectorAll('meta[name="color-scheme"]')).toHaveLength(1)
    expect(target.querySelectorAll('meta[name="theme-color"]')).toHaveLength(1)
    expect(target.documentElement.dataset.theme).toBe('day')
    expect(target.querySelector('meta[name="color-scheme"]')?.getAttribute('content')).toBe('light')
    expect(target.querySelector('meta[name="theme-color"]')?.getAttribute('content')).toBe(
      'day-chrome',
    )
  })

  it('creates missing metadata without inventing a browser chrome colour', () => {
    const target = emptyDocument()

    expect(() => applyDocumentTheme('night', target)).not.toThrow()

    expect(target.querySelector('meta[name="color-scheme"]')?.getAttribute('content')).toBe('dark')
    expect(target.querySelector('meta[name="theme-color"]')?.getAttribute('content')).toBe('')
  })

  it('is safe without a browser document', () => {
    expect(() => applyDocumentTheme('day', null)).not.toThrow()
  })
})
