import { useTranslation } from 'react-i18next'
import type { AppFailure } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib/localization'
import { TechnicalDetail } from '../technical-detail/TechnicalDetail'

/** Renders the allowlisted product explanation first and keeps optional operator diagnostics
 * behind a consistently labelled disclosure. React renders the diagnostic as inert text. */
export function AppFailureMessage({ failure }: { failure: AppFailure }) {
  const { t } = useTranslation(['errors', 'common'])
  return (
    <div className="min-w-0 break-words">
      <span>{formatAppFailure(failure)}</span>
      <TechnicalDetail
        label={t('failure.technicalDetail', { ns: 'common' })}
        detail={failure.technicalDetail}
      />
    </div>
  )
}
