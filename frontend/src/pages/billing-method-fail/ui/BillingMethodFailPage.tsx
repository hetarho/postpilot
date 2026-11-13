import { Link, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { isInAppPath } from '@/shared/lib'
import {
  Notice,
  ReturnIcon,
  TechnicalDetail,
  Typography,
  pageStyles,
  returnControlStyles,
} from '@/shared/ui'

export function BillingMethodFailPage() {
  const { t } = useTranslation(['billing', 'common'])
  const search = useSearch({ from: '/authenticated/billing/method/fail' })
  const detail = [search.code, search.message].filter(Boolean).join(': ')
  const back = isInAppPath(search.redirect) ? search.redirect : '/billing'
  const backLabel = t(
    back === '/billing'
      ? 'registration.back'
      : back.split('?')[0] === '/billing/checkout'
        ? 'registration.backCheckout'
        : 'registration.backPrevious',
    { ns: 'billing' },
  )

  return (
    <main className={pageStyles()}>
      <Link
        to={back}
        aria-label={backLabel}
        title={backLabel}
        className={returnControlStyles('mb-4')}
      >
        <ReturnIcon />
      </Link>
      <Typography variant="display">{t('registration.title', { ns: 'billing' })}</Typography>
      <Notice tone="danger" role="alert" className="mt-8">
        {t('registration.failed', { ns: 'billing' })}
      </Notice>
      <TechnicalDetail
        label={t('failure.technicalDetail', { ns: 'common' })}
        detail={detail || undefined}
      />
    </main>
  )
}
