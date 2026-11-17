import { useWritingTestTranslation } from '@/features/writing-test'
import {
  writingTestCandidateReadable,
  writingTestPayloadAvailable,
  type WritingTest,
  type WritingTestCandidate,
} from '@/entities/writing-test'
import { formatAppFailure } from '@/shared/lib/localization'
import { Disclosure, Typography } from '@/shared/ui'
import { writingTestCandidateLabel } from '../lib/candidate-label'
import { WritingTestObservationViewer } from './WritingTestObservationPair'
import { WritingTestPost } from './WritingTestPair'

/** Each blind contestant by its letter as waiting, working, done or failed with its normalized
 *  reason (MODEL-101). A photo observation test's list is the review's own observation viewer over
 *  every contestant: one held photo, one card per contestant, the switcher's dropdown naming each
 *  letter with its status and a card without observations stating where its contestant is. A
 *  writing test's done contestant opens in place to its post as the pair shows it. Once the payload
 *  is purged each letter shows its status alone. The reason is the identity-safe product message
 *  the wire already carries; no model name or diagnostic appears before the reveal (MODEL-32). */
export function WritingTestContestantStatus({
  test,
  ownerId,
}: {
  test: WritingTest
  ownerId?: string
}) {
  const { t } = useWritingTestTranslation()
  const unit = test.output === 'observation' ? 'observation' : 'post'
  const payloadAvailable = writingTestPayloadAvailable(test)
  const statusText = (candidate: WritingTestCandidate) =>
    `${t(`contestants.${unit}.${candidate.status}`)}${
      candidate.status === 'failed' && candidate.failure
        ? ` · ${formatAppFailure(candidate.failure)}`
        : ''
    }`
  const title = (
    <Typography variant="fieldTitle" as="h3">
      {t('contestants.title')}
    </Typography>
  )
  if (unit === 'observation' && payloadAvailable && test.photos?.length)
    return (
      <section aria-label={t('contestants.title')}>
        {title}
        <div className="mt-2">
          <WritingTestObservationViewer
            test={test}
            columns={test.candidates.map((candidate) => ({
              candidate,
              label: writingTestCandidateLabel(test, candidate.id),
              status: t(`contestants.observation.${candidate.status}`),
              note: candidate.status === 'succeeded' ? undefined : statusText(candidate),
            }))}
          />
        </div>
      </section>
    )
  return (
    <section aria-label={t('contestants.title')}>
      {title}
      <ul className="divide-divider mt-2 divide-y">
        {test.candidates.map((candidate) => {
          const label = writingTestCandidateLabel(test, candidate.id)
          const readable =
            unit === 'post' &&
            payloadAvailable &&
            candidate.status === 'succeeded' &&
            writingTestCandidateReadable(candidate)
          if (readable)
            return (
              <li key={candidate.id} className="min-w-0 py-1">
                <Disclosure
                  size="row"
                  headingLevel={4}
                  title={t('candidate', { label })}
                  aside={
                    <Typography variant="body" as="span" className="break-words">
                      {statusText(candidate)}
                    </Typography>
                  }
                >
                  <div className="pt-2 pb-4" lang={test.targetLanguage}>
                    <WritingTestPost
                      ownerId={ownerId}
                      test={test}
                      candidate={candidate}
                      label={label}
                    />
                  </div>
                </Disclosure>
              </li>
            )
          return (
            <li
              key={candidate.id}
              className="flex min-w-0 flex-wrap items-baseline justify-between gap-x-3 gap-y-1 py-2"
            >
              <Typography variant="label" as="span">
                {t('candidate', { label })}
              </Typography>
              <Typography variant="body" as="span" className="min-w-0 break-words">
                {statusText(candidate)}
              </Typography>
            </li>
          )
        })}
      </ul>
    </section>
  )
}
