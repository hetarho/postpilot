import { describe, expect, it } from 'vitest'
import { LOCALE_STORAGE_KEY } from '@/shared/lib/localization'
import { resolveLocale } from './locale'

function storage(value: string | null): Pick<Storage, 'getItem'> {
  return {
    getItem: (key) => (key === LOCALE_STORAGE_KEY ? value : null),
  }
}

describe('locale resolution', () => {
  it('prefers an exact stored override over browser languages', () => {
    expect(resolveLocale({ storage: storage('ko'), navigatorLanguages: ['en-US'] })).toBe('ko')
    expect(resolveLocale({ storage: storage('en'), navigatorLanguages: ['ko-KR'] })).toBe('en')
  })

  it('uses the first supported browser primary tag case-insensitively', () => {
    expect(resolveLocale({ storage: storage(null), navigatorLanguages: ['ja-JP', 'EN-us'] })).toBe(
      'en',
    )
    expect(resolveLocale({ storage: storage(null), navigatorLanguages: ['ko-KR', 'en-US'] })).toBe(
      'ko',
    )
  })

  it('ignores malformed stored values instead of repairing them', () => {
    expect(resolveLocale({ storage: storage('EN'), navigatorLanguages: ['en-US'] })).toBe('en')
    expect(resolveLocale({ storage: storage('en-US'), navigatorLanguages: ['ko-KR'] })).toBe('ko')
  })

  it('falls back through a denied storage API and unsupported browsers to Korean', () => {
    const denied = {
      getItem: () => {
        throw new DOMException('denied')
      },
    }
    expect(resolveLocale({ storage: denied, navigatorLanguages: ['en-GB'] })).toBe('en')
    expect(resolveLocale({ storage: denied, navigatorLanguages: ['ja-JP'] })).toBe('ko')
  })
})
