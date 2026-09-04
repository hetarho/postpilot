import { appFailureFromConnect } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'

/** Shared by all template mutations so stable reasons are translated consistently. */
export function templateErrorMessage(error: unknown): string {
  if (!error) return ''
  return formatAppFailure(appFailureFromConnect(error))
}
