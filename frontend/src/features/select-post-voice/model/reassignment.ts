import i18next from 'i18next'
import { isTerminal, type GenerationJob } from '@/entities/generation-job'
import { appFailureFromConnect } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'

/** Mirrors the server's reassignment gate (spec/policy/posts.md) so the picker can say why it is
 *  disabled instead of letting the request fail: a job or an undecided A/B result could still
 *  establish a baseline for the old voice. Returns '' when reassignment is allowed. */
export function reassignmentBlocker(post: {
  activeJob: Pick<GenerationJob, 'status'> | undefined
  pendingExperimentId: string
}): string {
  if (post.activeJob && !isTerminal(post.activeJob))
    return i18next.t('assignment.jobBlocked', { ns: 'voices' })
  if (post.pendingExperimentId) return i18next.t('assignment.experimentBlocked', { ns: 'voices' })
  return ''
}

/** The server's stable refusal, translated through the public application-failure contract. */
export function reassignmentFailureMessage(cause: unknown): string {
  const failure = appFailureFromConnect(cause)
  const message = formatAppFailure(failure)
  return failure.reason === 'VOICE_NOT_FOUND'
    ? i18next.t('assignment.notFoundDetail', {
        ns: 'voices',
        error: message,
        interpolation: { escapeValue: false },
      })
    : message
}
