import { useState, type FormEvent } from 'react'
import { Link, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useRequestPasswordReset } from '@/features/request-password-reset'
import {
  AppFailureMessage,
  Button,
  FieldLabel,
  Notice,
  TextField,
  Typography,
  typographyStyles,
} from '@/shared/ui'

export function ForgotPasswordPage() {
  const { t } = useTranslation('auth')
  const { redirect } = useSearch({ from: '/forgot-password' })
  const request = useRequestPasswordReset()
  const [email, setEmail] = useState('')
  const [sent, setSent] = useState(false)

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    request.mutate({ email }, { onSuccess: () => setSent(true) })
  }

  return (
    <main className="bg-surface-base text-content-primary flex min-h-full items-center justify-center px-4 py-10 sm:px-6">
      <section className="w-full max-w-sm" aria-labelledby="forgot-password-heading">
        <Typography variant="display" as="h1" id="forgot-password-heading" className="text-center">
          {sent ? t('forgotPassword.sentHeading') : t('forgotPassword.heading')}
        </Typography>
        {sent ? (
          <div className="mt-5 grid gap-4 text-center">
            <Typography variant="body" className="text-content-secondary break-words">
              {t('forgotPassword.sentBody', { email })}
            </Typography>
            <Link
              to="/login"
              search={redirect ? { redirect } : {}}
              className={typographyStyles({
                variant: 'label',
                className:
                  'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center justify-center underline',
              })}
            >
              {t('links.login')}
            </Link>
          </div>
        ) : (
          <>
            <Typography variant="body" className="text-content-secondary mt-3 text-center">
              {t('forgotPassword.intro')}
            </Typography>
            <form onSubmit={onSubmit} className="mt-6 grid gap-3">
              <div>
                <FieldLabel htmlFor="reset-email">{t('field.email')}</FieldLabel>
                <TextField
                  id="reset-email"
                  type="text"
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  autoComplete="email"
                  autoCapitalize="none"
                  autoCorrect="off"
                  spellCheck={false}
                  required
                  autoFocus
                  className="mt-1.5"
                />
              </div>
              {request.failure && (
                <Notice tone="danger" role="alert">
                  <AppFailureMessage failure={request.failure} />
                </Notice>
              )}
              <Button type="submit" variant="cta" pending={request.isPending}>
                {t('forgotPassword.submit')}
              </Button>
            </form>
          </>
        )}
      </section>
    </main>
  )
}
