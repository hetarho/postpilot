import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import type { AppFailure } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import { TechnicalDetail, Typography, buttonStyles } from '@/shared/ui'

/** One refusal, as the clip screens state it: the server's stable reason in the
 *  owner's words, the technical detail folded away, and — where the refusal is a
 *  balance — the way to buy more beside it. */
export function ClipFailureNotice({ failure }: { failure?: AppFailure }) {
  const { t } = useTranslation(['clips', 'common'])
  if (!failure) return null
  return (
    <div role="alert" className="mb-3 space-y-2">
      <Typography variant="body">{formatAppFailure(failure)}</Typography>
      <TechnicalDetail
        label={t('failure.technicalDetail', { ns: 'common' })}
        detail={failure.technicalDetail}
      />
      {failure.reason === 'INSUFFICIENT_CREDITS' && (
        <Link to="/plans" className={buttonStyles({ variant: 'secondary' })}>
          {t('generation.plans', { ns: 'clips' })}
        </Link>
      )}
    </div>
  )
}
