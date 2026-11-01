import { beforeEach, describe, expect, it } from 'vitest'
import {
  offerDismissalKey,
  readOfferDismissed,
  resetOfferDismissalMemory,
  writeOfferDismissed,
  type OfferStorage,
} from './dismissal'

function memoryStorage(): OfferStorage & { entries: Map<string, string> } {
  const entries = new Map<string, string>()
  return {
    entries,
    getItem: (key) => entries.get(key) ?? null,
    setItem: (key, value) => void entries.set(key, value),
  }
}
const throwing: OfferStorage = {
  getItem: () => {
    throw new Error('blocked')
  },
  setItem: () => {
    throw new Error('blocked')
  },
}

beforeEach(() => {
  resetOfferDismissalMemory()
})

describe('the offer dismissal', () => {
  it('is kept per account and per kind', () => {
    const storage = memoryStorage()
    writeOfferDismissed('alice', 'post', storage)

    expect(readOfferDismissed('alice', 'post', storage)).toBe(true)
    expect(readOfferDismissed('alice', 'clip', storage)).toBe(false)
    expect(readOfferDismissed('bob', 'post', storage)).toBe(false)
    expect(storage.entries.get(offerDismissalKey('alice', 'post'))).toBe('1')
  })

  it('survives a fresh session through browser storage', () => {
    const storage = memoryStorage()
    writeOfferDismissed('alice', 'clip', storage)
    resetOfferDismissalMemory()

    expect(readOfferDismissed('alice', 'clip', storage)).toBe(true)
  })

  it('still holds for this session when browser storage throws', () => {
    expect(() => writeOfferDismissed('alice', 'post', throwing)).not.toThrow()
    expect(readOfferDismissed('alice', 'post', throwing)).toBe(true)
    expect(readOfferDismissed('alice', 'clip', throwing)).toBe(false)
  })

  it('records nothing without an account', () => {
    const storage = memoryStorage()
    writeOfferDismissed('', 'post', storage)

    expect(storage.entries.size).toBe(0)
    expect(readOfferDismissed('', 'post', storage)).toBe(false)
  })

  it('escapes the account id inside its key', () => {
    expect(offerDismissalKey('a/b c', 'post')).toBe('postpilot.creation-offer.v1.post.a%2Fb%20c')
  })
})
