import { Link, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { isInAppPath } from '@/shared/lib'
import { Notice, TechnicalDetail, Typography, pageStyles, typographyStyles } from '@/shared/ui'

export function BillingMethodFailPage() {
  const { t } = useTranslation(['billing', 'common'])
  const search = useSearch({ from: '/authenticated/billing/method/fail' })
  const detail = [search.code, search.message].filter(Boolean).join(': ')
  const back = isInAppPath(search.redirect) ? search.redirect : '/billing'

  return (
    <main className={pageStyles()}>
      <Typography variant="display">{t('registration.title', { ns: 'billing' })}</Typography>
      <Notice tone="danger" role="alert" className="mt-8">
        {t('registration.failed', { ns: 'billing' })}
      </Notice>
      <TechnicalDetail
        label={t('failure.technicalDetail', { ns: 'common' })}
        detail={detail || undefined}
      />
      <Link
        to={back}
        className={typographyStyles({
          variant: 'label',
          className:
            'text-link-fg hover:text-link-fg-hover mt-5 inline-flex min-h-11 items-center underline',
        })}
      >
        {t('registration.back', { ns: 'billing' })}
      </Link>
    </main>
  )
}
