import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import type { AppFailure } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import { TechnicalDetail, Typography, buttonStyles } from '@/shared/ui'

export function ClipGenerationFailure({ failure }: { failure?: AppFailure }) {
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
