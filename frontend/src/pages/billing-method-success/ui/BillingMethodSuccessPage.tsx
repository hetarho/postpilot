import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { useRegisterPaymentMethod } from '@/entities/subscription'
import { isInAppPath } from '@/shared/lib'
import {
  Notice,
  ReturnIcon,
  Spinner,
  Typography,
  pageStyles,
  returnControlStyles,
} from '@/shared/ui'

export function BillingMethodSuccessPage() {
  const { t } = useTranslation('billing')
  const search = useSearch({ from: '/authenticated/billing/method/success' })
  const navigate = useNavigate()
  const registration = useRegisterPaymentMethod()
  const started = useRef(false)
  const valid = Boolean(search.authKey && search.customerKey)

  useEffect(() => {
    if (started.current || !search.authKey || !search.customerKey) return
    started.current = true
    void registration
      .register(search.authKey, search.customerKey)
      .then((response) => {
        if (!response.paymentMethod) return
        if (isInAppPath(search.redirect)) {
          void navigate({ to: search.redirect, replace: true })
          return
        }
        void navigate({
          to: '/billing',
          replace: true,
          state: (previous) => ({
            ...previous,
            billingRegistration: {
              cardLabel: response.paymentMethod?.cardLabel ?? '',
            },
          }),
        })
      })
      .catch(() => {
        // The mutation's catalog-backed message replaces the progress state.
      })
  }, [navigate, registration, search.authKey, search.customerKey, search.redirect])

  return (
    <main className={pageStyles()}>
      <Link
        to="/billing"
        aria-label={t('registration.back')}
        title={t('registration.back')}
        className={returnControlStyles('mb-4')}
      >
        <ReturnIcon />
      </Link>
      <Typography variant="display">{t('registration.title')}</Typography>
      {valid && !registration.isError ? (
        <div role="status" className="mt-8 flex items-center gap-3">
          <Spinner />
          <Typography variant="body">{t('registration.checking')}</Typography>
        </div>
      ) : (
        <>
          <Notice tone="danger" role="alert" className="mt-8">
            {registration.errorMessage || t('registration.invalid')}
          </Notice>
        </>
      )}
    </main>
  )
}
