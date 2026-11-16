import { appFailureFromConnect } from '@/shared/api'
import { formatAppFailure } from './failure'

/** The user-facing text of a failed Connect call; '' while there is no error. */
export function connectFailureMessage(error: unknown): string {
  return error ? formatAppFailure(appFailureFromConnect(error)) : ''
}
