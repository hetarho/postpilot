import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useTransport } from '@connectrpc/connect-query'
import { FailureNotice, ProgressLine, isTerminal, useJob } from '@/entities/generation-job'
import { voiceProfileQueryKey } from '@/entities/voice'

/** The one durable run that can be writing THIS voice's profile: an analysis, or the seeding job
 *  a described creation started. Both are reported the same way because they end the same way —
 *  a new profile version, or a failure the voice survives — and the profile query is refetched
 *  when either finishes. Rendering nothing at all while a job runs is the shape a user reads as
 *  "the button did nothing". */
export function VoiceRunStatus({
  ownerId,
  voiceId,
  jobId,
}: {
  ownerId: string
  voiceId: string
  jobId: string
}) {
  const { t } = useTranslation('voices')
  const transport = useTransport()
  const invalidateOnDone = useMemo(
    () => [voiceProfileQueryKey(transport, ownerId, voiceId)],
    [ownerId, transport, voiceId],
  )
  const jobState = useJob(jobId, invalidateOnDone)
  if (!jobId) return null
  return (
    <section className="mt-6" aria-label={t('screens.analysisStatus')}>
      {jobState.isError ? (
        <FailureNotice message={t('screens.analysisStatusFailed')} onRetry={jobState.refetch} />
      ) : jobState.job?.status === 'failed' ? (
        <FailureNotice failure={jobState.job.failure} />
      ) : jobState.job && !isTerminal(jobState.job) ? (
        <ProgressLine job={jobState.job} />
      ) : null}
    </section>
  )
}
