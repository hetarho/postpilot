import { useWritingTestTranslation } from '@/features/writing-test'
import type { WritingTest } from '@/entities/writing-test'
import { Typography } from '@/shared/ui'

/** Render stored server matches only; waiting slots never acquire an inferred contestant or champion. */
export function WritingTestBracket({ test }: { test: WritingTest }) {
  const { t } = useWritingTestTranslation()
  const matches = [...test.matches].sort(
    (left, right) => left.round - right.round || left.index - right.index,
  )
  const labelOf = (id: string) => {
    const index = test.candidates.findIndex((candidate) => candidate.id === id)
    return index >= 0 ? String.fromCharCode(65 + index) : ''
  }
  return (
    <section aria-label={t('bracket')}>
      <Typography variant="title" as="h2">
        {t('bracket')}
      </Typography>
      <Typography variant="label" as="p" role="status" className="mt-2">
        {t('matchProgress', {
          done: matches.filter((match) => !!match.winnerCandidateId).length,
          total: test.count - 1,
        })}
      </Typography>
      {matches.length === 0 && (
        <Typography variant="body" as="p" className="mt-3">
          {t('waitingMatch')}
        </Typography>
      )}
      <ol className="divide-divider mt-3 divide-y">
        {matches.map((match) => (
          <li key={match.id} className="py-3">
            <Typography variant="label" as="p">
              {t('round', { round: match.round, number: match.index + 1 })}
            </Typography>
            {match.leftCandidateId && match.rightCandidateId ? (
              <Typography variant="body" as="p" className="mt-1">
                {t('candidate', { label: labelOf(match.leftCandidateId) })} ·{' '}
                {t('candidate', { label: labelOf(match.rightCandidateId) })}
              </Typography>
            ) : (
              <Typography variant="body" as="p" className="text-content-secondary mt-1">
                {t('waitingMatch')}
              </Typography>
            )}
            {match.winnerCandidateId && (
              <Typography variant="label" as="p" className="mt-1">
                {t('decided', {
                  label: t('candidate', { label: labelOf(match.winnerCandidateId) }),
                })}
              </Typography>
            )}
          </li>
        ))}
      </ol>
    </section>
  )
}
