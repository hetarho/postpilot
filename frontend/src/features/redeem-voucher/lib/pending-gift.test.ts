import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { PENDING_GIFT_KEY, PENDING_GIFT_TTL_MS } from '../config/storage'
import { clearPendingGift, readPendingGift, savePendingGift, takePendingGift } from './pending-gift'

describe('pending gift', () => {
  beforeEach(() => localStorage.clear())
  afterEach(() => {
    vi.restoreAllMocks()
    localStorage.clear()
  })

  it('remembers a gift link while it is fresh and forgets it after a day', () => {
    savePendingGift('tok', 1_000)
    expect(readPendingGift(1_000 + PENDING_GIFT_TTL_MS)).toBe('tok')
    expect(readPendingGift(1_000 + PENDING_GIFT_TTL_MS + 1)).toBeUndefined()
    // A clock that moved backwards is not trusted either.
    expect(readPendingGift(999)).toBeUndefined()
  })

  it('hands a pending gift back once', () => {
    savePendingGift('tok', 1_000)
    expect(takePendingGift(2_000)).toBe('tok')
    expect(takePendingGift(2_000)).toBeUndefined()
    expect(localStorage.getItem(PENDING_GIFT_KEY)).toBeNull()
  })

  it('reads a malformed entry as nothing', () => {
    localStorage.setItem(PENDING_GIFT_KEY, '{not json')
    expect(readPendingGift()).toBeUndefined()
    localStorage.setItem(PENDING_GIFT_KEY, JSON.stringify({ token: '', savedAt: 1 }))
    expect(readPendingGift(2)).toBeUndefined()
  })

  it('treats storage that throws as no pending gift', () => {
    // The way a browser with site data blocked behaves: touching storage at all throws.
    vi.spyOn(window, 'localStorage', 'get').mockImplementation(() => {
      throw new DOMException('blocked', 'SecurityError')
    })
    expect(() => savePendingGift('tok')).not.toThrow()
    expect(readPendingGift()).toBeUndefined()
    expect(() => clearPendingGift()).not.toThrow()
    expect(takePendingGift()).toBeUndefined()
  })
})
