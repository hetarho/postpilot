import { useWritingTestTranslation } from '@/features/writing-test'
import { writingTestPayloadAvailable, type WritingTest } from '@/entities/writing-test'
import { Notice, Typography } from '@/shared/ui'
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
