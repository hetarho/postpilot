import { appFailureFromConnect } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'

/** Shared by every memory mutation so stable reasons are translated consistently. The server's
 *  message is what the user reads — the client rephrases no refusal and predicts none, which is
 *  why the account cap is not mirrored here (MEM-11). */
export function memoryErrorMessage(error: unknown): string {
  if (!error) return ''
  return formatAppFailure(appFailureFromConnect(error))
}

/** True when the account is at its memory cap. Nothing is evicted to make room, so the surface
 *  that meets this says what to do (delete one) rather than retrying. */
export function isMemoryLimitReached(error: unknown): boolean {
  if (!error) return false
  return appFailureFromConnect(error).reason === 'MEMORY_LIMIT_REACHED'
}
