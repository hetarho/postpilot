import { PENDING_GIFT_KEY, PENDING_GIFT_TTL_MS } from '../config/storage'

interface PendingGift {
  token: string
  savedAt: number
}

/** Remembers the gift link an anonymous visitor opened, so signing in — even through the
 *  email-verification detour — can bring them back to it. Browser storage is optional: when it
 *  is blocked the visitor reopens the link from the message it came in. */
export function savePendingGift(token: string, now = Date.now()): void {
  try {
    const value: PendingGift = { token, savedAt: now }
    window.localStorage.setItem(PENDING_GIFT_KEY, JSON.stringify(value))
  } catch {
    // Storage is optional; the gift page still works without it.
  }
}

/** The pending gift's token while it is fresh, otherwise nothing. */
export function readPendingGift(now = Date.now()): string | undefined {
  try {
    const raw = window.localStorage.getItem(PENDING_GIFT_KEY)
    if (!raw) return undefined
    const value = JSON.parse(raw) as Partial<PendingGift>
    if (
      typeof value.token !== 'string' ||
      value.token === '' ||
      typeof value.savedAt !== 'number'
    ) {
      return undefined
    }
    if (now - value.savedAt > PENDING_GIFT_TTL_MS || now < value.savedAt) return undefined
    return value.token
  } catch {
    return undefined
  }
}

export function clearPendingGift(): void {
  try {
    window.localStorage.removeItem(PENDING_GIFT_KEY)
  } catch {
    // Nothing to clear when storage is blocked.
  }
}

/** Reads the pending gift once and forgets it, fresh or not: a hand-back happens at most once. */
export function takePendingGift(now = Date.now()): string | undefined {
  const token = readPendingGift(now)
  clearPendingGift()
  return token
}
