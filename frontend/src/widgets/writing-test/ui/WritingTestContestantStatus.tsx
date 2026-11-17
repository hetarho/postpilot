import { useWritingTestTranslation } from '@/features/writing-test'
import type { WritingTest } from '@/entities/writing-test'
import { formatAppFailure } from '@/shared/lib/localization'
import { Typography } from '@/shared/ui'

/** Each blind contestant by its letter as waiting, working, done or failed with its normalized
 *  reason (MODEL-101). The reason is the identity-safe product message the wire already carries;
 *  no model name or diagnostic appears before the reveal (MODEL-32). */
export function WritingTestContestantStatus({ test }: { test: WritingTest }) {
  const { t } = useWritingTestTranslation()
  const unit = test.output === 'observation' ? 'observation' : 'post'
  return (
    <section aria-label={t('contestants.title')}>
      <Typography variant="fieldTitle" as="h3">
        {t('contestants.title')}
      </Typography>
      <ul className="divide-divider mt-2 divide-y">
        {test.candidates.map((candidate, index) => (
          <li
            key={candidate.id}
            className="flex min-w-0 flex-wrap items-baseline justify-between gap-x-3 gap-y-1 py-2"
          >
            <Typography variant="label" as="span">
              {t('candidate', { label: String.fromCharCode(65 + index) })}
            </Typography>
            <Typography variant="body" as="span" className="min-w-0 break-words">
              {t(`contestants.${unit}.${candidate.status}`)}
              {candidate.status === 'failed' && candidate.failure
                ? ` · ${formatAppFailure(candidate.failure)}`
                : ''}
            </Typography>
          </li>
        ))}
      </ul>
    </section>
  )
}
