import i18next from 'i18next'
import { appFailureFromConnect } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'

/** What the editor says beside the select while a job is running.
 *
 *  Not a blocker, unlike a voice reassignment: a template is never learned from, so changing
 *  it is allowed in every status and while a job is in flight (spec/policy/templates.md). The
 *  running job keeps the brief frozen at its enqueue, and the note is there so nobody expects
 *  the change to reach work already queued. */
export function runningJobNote(): string {
  return i18next.t('assignment.runningJob', { ns: 'templates' })
}

/** The server's stable refusal, translated through the public application-failure contract. */
export function assignmentFailureMessage(cause: unknown): string {
  const failure = appFailureFromConnect(cause)
  const message = formatAppFailure(failure)
  return failure.reason === 'TEMPLATE_NOT_FOUND'
    ? i18next.t('assignment.notFoundDetail', {
        ns: 'templates',
        error: message,
        interpolation: { escapeValue: false },
      })
    : message
}
