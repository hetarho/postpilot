import { useWritingTestTranslation } from '@/features/writing-test'
import {
  writingTestCandidateReadable,
  writingTestPayloadAvailable,
  type WritingTest,
  type WritingTestCandidate,
} from '@/entities/writing-test'
import { Notice, Typography } from '@/shared/ui'
import { writingTestCandidateLabel } from '../lib/candidate-label'
import { WritingTestObservationRows } from './WritingTestObservationPair'
import { WritingTestPost } from './WritingTestPair'

/** Read and copy the server-confirmed champion without applying it or creating a new match. */
export function WritingTestChampionOutput({
  test,
  ownerId,
}: {
  test: WritingTest
  ownerId?: string
}) {
  const { t } = useWritingTestTranslation()
  if (test.status !== 'completed' || !test.revealed || !test.winnerCandidateId) return null
  const champion = test.candidates.find((candidate) => candidate.id === test.winnerCandidateId)
  if (!champion || champion.status !== 'succeeded')
    return (
      <Notice tone="warning" role="status">
        {t('failed')}
      </Notice>
    )
  if (test.output === 'observation') return <ChampionObservations test={test} champion={champion} />
  if (!champion.output || !writingTestPayloadAvailable(test))
    return (
      <Notice tone="warning" role="status">
        {t('expired')}
      </Notice>
    )
  const label = String.fromCharCode(
    65 + test.candidates.findIndex((candidate) => candidate.id === champion.id),
  )
  return (
    <section aria-label={t('winningPost')} lang={test.targetLanguage}>
      <Typography variant="title" as="h2">
        {t('winningPost')}
      </Typography>
      <Typography variant="label" as="p" className="mt-2">
        {t('frozenLanguage', { language: t(test.targetLanguage) })}
      </Typography>
      <WritingTestPost
        ownerId={ownerId}
        key={`${test.id}:${champion.id}`}
        test={test}
        candidate={champion}
        label={label}
      />
    </section>
  )
}

/** A photo test's champion is read, never applied to a post or exported (MODEL-97). Once its
 *  private payload is purged only the retained metadata remains, under the expired notice. */
function ChampionObservations({
  test,
  champion,
}: {
  test: WritingTest
  champion: WritingTestCandidate
}) {
  const { t } = useWritingTestTranslation()
  const label = writingTestCandidateLabel(test, champion.id)
  const readable = writingTestCandidateReadable(champion) && writingTestPayloadAvailable(test)
  const photoCount = test.photos?.length ?? 0
  return (
    <section aria-label={t('observation.winningObservation')}>
      <Typography variant="title" as="h2">
        {t('observation.winningObservation')}
      </Typography>
      <Typography variant="label" as="p" className="mt-2 break-words">
        {t('candidate', { label })}
        {champion.identity ? ` · ${champion.identity.label}` : ''}
      </Typography>
      {readable ? (
        <div className="mt-4">
          <WritingTestObservationRows test={test} columns={[{ candidate: champion, label }]} />
        </div>
      ) : (
        <>
          {photoCount > 0 && (
            <Typography variant="label" as="p" className="mt-2">
              {t('observation.photoCount', { count: photoCount })}
            </Typography>
          )}
          <Notice tone="warning" role="status" className="mt-4">
            {t('observation.expired')} · {t('observation.expiredHelp')}
          </Notice>
        </>
      )}
    </section>
  )
}
