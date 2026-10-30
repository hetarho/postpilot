import { describe, expect, it } from 'vitest'
import type { ParsedLocation } from '@tanstack/react-router'
import { router } from './router'
import { scrollRestorationKey } from './scroll-restoration'

function at(href: string, entryKey = 'entry-1'): ParsedLocation {
  const url = new URL(href, 'https://postpilot.test')
  return {
    href: `${url.pathname}${url.search}`,
    pathname: url.pathname,
    state: { __TSR_key: entryKey },
  } as unknown as ParsedLocation
}

describe('scrollRestorationKey', () => {
  // POST-93: the list keeps its place however it is reached again — back, or the editor's 목록,
  // which is a new history entry — and each narrowing keeps its own.
  it('keys the post history (/naver-blog) by its address, not by the history entry', () => {
    expect(scrollRestorationKey(at('/naver-blog', 'a'))).toBe(
      scrollRestorationKey(at('/naver-blog', 'b')),
    )
    expect(scrollRestorationKey(at('/naver-blog?q=제주', 'a'))).not.toBe(
      scrollRestorationKey(at('/naver-blog', 'a')),
    )
  })

  // Everywhere else a new navigation starts at the top: only its own entry is restored.
  it('keys every other screen by its history entry', () => {
    expect(scrollRestorationKey(at('/posts/20260828-jeju', 'a'))).toBe('a')
    expect(scrollRestorationKey(at('/posts/20260828-jeju', 'b'))).toBe('b')
    expect(scrollRestorationKey(at('/naver-clip', 'c'))).toBe('c')
  })
})

// The app's router is the one that restores: every screen, keyed as above. A per-location
// predicate would also skip the reset to the top on every screen it turns down.
it('is what the app router restores scroll with', () => {
  expect(router.options.scrollRestoration).toBe(true)
  expect(router.options.getScrollRestorationKey).toBe(scrollRestorationKey)
})
