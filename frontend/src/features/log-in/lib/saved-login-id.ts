import { SAVED_LOGIN_ID_KEY } from '../config/storage'

export function readSavedLoginId(): string {
  try {
    return window.localStorage.getItem(SAVED_LOGIN_ID_KEY) ?? ''
  } catch {
    return ''
  }
}

export function saveLoginId(id: string): void {
  try {
    if (id) window.localStorage.setItem(SAVED_LOGIN_ID_KEY, id)
    else window.localStorage.removeItem(SAVED_LOGIN_ID_KEY)
  } catch {
    // Browser storage is optional; signing in must still work when it is blocked.
  }
}
