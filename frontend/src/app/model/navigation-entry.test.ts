import { expect, it } from 'vitest'
import {
  entryHref,
  navigationParent,
  readNavigationEntry,
  rememberNavigationEntry,
} from './navigation-entry'

function memory() {
  const values = new Map<string, string>()
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => values.set(key, value),
    removeItem: (key: string) => values.delete(key),
  }
}
it('retains safe parent filters and exact owner/child identity without carrying object state', () => {
  const storage = memory()
  expect(
    rememberNavigationEntry(
      'alice',
      '/templates/item',
      '/templates?q=food&status=active&secret=ignored',
      'settings',
      720,
      storage,
    ),
  ).toBe(true)
  const saved = readNavigationEntry('alice', '/templates/item', storage)!
  expect(entryHref(saved)).toBe('/templates?q=food&status=active')
  expect(saved.scrollY).toBe(720)
  expect(readNavigationEntry('bob', '/templates/item', storage)).toBeUndefined()
  expect(readNavigationEntry('alice', '/templates/other', storage)).toBeUndefined()
})
it('refuses external/malformed/unproven record parents and clears unsafe filters', () => {
  const storage = memory()
  for (const value of [
    'https://example.test',
    '//example.test',
    '/\\evil.test',
    '/posts/foreign',
    '/settings%252f%252fevil.test',
  ]) {
    expect(navigationParent(value)).toBeUndefined()
    expect(rememberNavigationEntry('alice', '/tests', value, 'tests', 0, storage)).toBe(false)
  }
  expect(navigationParent('/settings#settings-writing')).toEqual({
    path: '/settings#settings-writing',
    filters: {},
  })
  expect(navigationParent('/tests/history?stage=write&source=owned')).toEqual({
    path: '/tests/history',
    filters: { stage: 'write', source: 'owned' },
  })
})

it('separates root switches from structural and explicit workflow entry', () => {
  const storage = memory()
  for (const target of ['/', '/tests', '/settings', '/library'])
    expect(rememberNavigationEntry('alice', target, '/posts', 'history', 100, storage, true)).toBe(
      false,
    )
  expect(rememberNavigationEntry('alice', '/templates/item', '/tests', 'tests', 100, storage)).toBe(
    false,
  )
  expect(
    rememberNavigationEntry(
      'alice',
      '/tests/result',
      '/tests/history?stage=write',
      'tests',
      100,
      storage,
    ),
  ).toBe(true)
  expect(
    rememberNavigationEntry(
      'alice',
      '/tests/records/result',
      '/posts?q=food',
      'history',
      100,
      storage,
      true,
    ),
  ).toBe(true)
  expect(entryHref(readNavigationEntry('alice', '/tests/records/result', storage)!)).toBe(
    '/posts?q=food',
  )
  expect(navigationParent('/posts#settings-writing')?.path).toBe('/posts')
  expect(navigationParent('/settings#settings-unknown')?.path).toBe('/settings')
})
