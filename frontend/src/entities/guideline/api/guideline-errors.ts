import { appFailureFromConnect } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'

/** Shared by every guideline mutation so stable reasons are translated consistently. The server's
 *  message is what the user reads — no client-side rephrasing of a refusal. */
export function guidelineErrorMessage(error: unknown): string {
  if (!error) return ''
  return formatAppFailure(appFailureFromConnect(error))
}

/** True when a create was refused because the account already holds that exact text. It is
 *  information ("already saved"), not a failure state, wherever a capture offers to save one. */
export function isDuplicateGuideline(error: unknown): boolean {
  if (!error) return false
  return appFailureFromConnect(error).reason === 'GUIDELINE_TEXT_TAKEN'
}
