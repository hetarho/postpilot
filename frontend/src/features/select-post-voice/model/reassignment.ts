import i18next from 'i18next'
import { isTerminal, type GenerationJob } from '@/entities/generation-job'
import { appFailureFromConnect } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'

/** Ordinary writing holds the current assignment. Queued/frozen tests carry their own
 *  settings and retained results never prevent choosing a voice for future writing. */
export function reassignmentBlocker(post: {
  activeJob: Pick<GenerationJob, 'status'> | undefined
  pendingExperimentId: string
}): string {
  if (post.activeJob && !isTerminal(post.activeJob))
    return i18next.t('assignment.jobBlocked', { ns: 'voices' })
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
