import { appFailureFromConnect } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'

/** Shared by all purpose mutations so stable reasons are translated consistently. */
export function purposeErrorMessage(error: unknown): string {
  if (!error) return ''
  return formatAppFailure(appFailureFromConnect(error))
}
