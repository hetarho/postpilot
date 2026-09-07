import { useEffect, useState, type FormEvent } from 'react'
import { Link, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useResendVerification, useVerifyEmail } from '@/features/verify-email'
import {
  AppFailureMessage,
  Button,
  FieldLabel,
  Notice,
  TextField,
  Typography,
  buttonStyles,
} from '@/shared/ui'

export function VerifyEmailPage() {
  const { t } = useTranslation('auth')
  const { token } = useSearch({ from: '/verify-email' })
  const verify = useVerifyEmail()
  const resend = useResendVerification()
  const [status, setStatus] = useState<'checking' | 'success' | 'failed'>('checking')
  const [email, setEmail] = useState('')
  const [resent, setResent] = useState(false)

  const verifyMutation = verify.mutate
  useEffect(() => {
    verifyMutation(
      { token: token ?? '' },
      { onSuccess: () => setStatus('success'), onError: () => setStatus('failed') },
    )
  }, [token, verifyMutation])

  const onResend = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    resend.mutate({ email }, { onSuccess: () => setResent(true) })
  }

  return (
    <main className="bg-surface-base text-content-primary flex min-h-full items-center justify-center px-4 py-10 sm:px-6">
      <section className="w-full max-w-sm text-center" aria-labelledby="verify-title">
        <Typography variant="display" as="h1" id="verify-title">
          {status === 'checking' && t('verification.checking')}
          {status === 'success' && t('verification.successHeading')}
          {status === 'failed' && t('verification.failedHeading')}
        </Typography>
        {status === 'checking' && (
          <Typography variant="body" className="text-content-secondary mt-3">
            {t('verification.checkingBody')}
          </Typography>
        )}
        {status === 'success' && (
          <>
            <Typography variant="body" className="text-content-secondary mt-3">
              {t('verification.successBody')}
            </Typography>
            <Link to="/login" className={buttonStyles({ variant: 'cta', className: 'mt-6' })}>
              {t('links.login')}
            </Link>
          </>
        )}
        {status === 'failed' && (
          <div className="mt-5 grid gap-4 text-left">
            {verify.failure && (
              <Notice tone="danger" role="alert">
                <AppFailureMessage failure={verify.failure} />
              </Notice>
            )}
            {resent ? (
              <Notice tone="success" role="status">
                {t('verification.resent', { email })}
              </Notice>
            ) : (
              <form onSubmit={onResend} className="grid gap-3">
                <FieldLabel htmlFor="verify-resend-email">{t('field.email')}</FieldLabel>
                <TextField
                  id="verify-resend-email"
                  type="text"
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  autoComplete="email"
                  autoCapitalize="none"
                  required
                />
                {resend.failure && (
                  <Notice tone="danger" role="alert">
                    <AppFailureMessage failure={resend.failure} />
                  </Notice>
                )}
                <Button type="submit" variant="cta" pending={resend.isPending}>
                  {t('verification.resend')}
                </Button>
              </form>
            )}
          </div>
        )}
      </section>
    </main>
  )
}
