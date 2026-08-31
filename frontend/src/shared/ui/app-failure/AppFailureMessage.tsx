import { useTranslation } from 'react-i18next'
import type { AppFailure } from '@/shared/api'
import { TechnicalDetail } from '../technical-detail/TechnicalDetail'

/** Renders the allowlisted product explanation first and keeps optional operator diagnostics
 * behind a consistently labelled disclosure. React renders the diagnostic as inert text. */
export function AppFailureMessage({ failure }: { failure: AppFailure }) {
  const { t } = useTranslation(['errors', 'common'])
  const translateFailure = t as unknown as (
    key: string,
    options: Readonly<Record<string, string | undefined>>,
  ) => string
  return (
    <div className="min-w-0 break-words">
      <span>{translateFailure(failure.reason, { ns: 'errors', ...failure.params })}</span>
      <TechnicalDetail
        label={t('failure.technicalDetail', { ns: 'common' })}
        detail={failure.technicalDetail}
      />
    </div>
  )
}
