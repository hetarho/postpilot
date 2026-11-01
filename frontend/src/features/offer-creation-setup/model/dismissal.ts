import { CREATION_OFFER_DISMISSAL_PREFIX } from '../config'
import type { CreationOfferKind } from './offer'

/** `다시 묻지 않기`, per account and kind on this browser (POST-121). Kept here as well, so a
 *  browser that rejects storage still stops asking for the rest of the session. */
const memoryDismissals = new Set<string>()

export function resetOfferDismissalMemory(): void {
  memoryDismissals.clear()
}

export interface OfferStorage {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
}

export function offerDismissalKey(ownerId: string, kind: CreationOfferKind): string {
  return CREATION_OFFER_DISMISSAL_PREFIX + kind + '.' + encodeURIComponent(ownerId)
}

function browserOfferStorage(): OfferStorage | null {
  try {
    return window.localStorage
  } catch {
    return null
  }
}

export function readOfferDismissed(
  ownerId: string,
  kind: CreationOfferKind,
  storage: OfferStorage | null = browserOfferStorage(),
): boolean {
  if (!ownerId) return false
  const key = offerDismissalKey(ownerId, kind)
  if (memoryDismissals.has(key)) return true
  try {
    return storage?.getItem(key) === '1'
  } catch {
    return false
  }
}

export function writeOfferDismissed(
  ownerId: string,
  kind: CreationOfferKind,
  storage: OfferStorage | null = browserOfferStorage(),
): void {
  if (!ownerId) return
  const key = offerDismissalKey(ownerId, kind)
  memoryDismissals.add(key)
  try {
    storage?.setItem(key, '1')
  } catch {
    /* The memory copy keeps the answer for this session when browser storage rejects writes. */
  }
}
