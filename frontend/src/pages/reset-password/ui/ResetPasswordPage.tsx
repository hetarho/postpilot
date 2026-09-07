import { useState, type FormEvent } from 'react'
import { Link, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useResetPassword } from '@/features/reset-password'
import {
  AppFailureMessage,
  Button,
  FieldLabel,
  Notice,
  TextField,
  Typography,
  buttonStyles,
  typographyStyles,
} from '@/shared/ui'

export function ResetPasswordPage() {
  const { t } = useTranslation('auth')
  const { token } = useSearch({ from: '/reset-password' })
  const reset = useResetPassword()
  const [newPassword, setNewPassword] = useState('')
  const [changed, setChanged] = useState(false)

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    reset.mutate({ token: token ?? '', newPassword }, { onSuccess: () => setChanged(true) })
  }

  return (
    <main className="bg-surface-base text-content-primary flex min-h-full items-center justify-center px-4 py-10 sm:px-6">
      <section className="w-full max-w-sm" aria-labelledby="reset-password-heading">
        <Typography variant="display" as="h1" id="reset-password-heading" className="text-center">
          {changed ? t('resetPassword.successHeading') : t('resetPassword.heading')}
        </Typography>
        {changed ? (
          <div className="mt-5 grid gap-4 text-center">
            <Typography variant="body" className="text-content-secondary">
              {t('resetPassword.successBody')}
            </Typography>
            <Link to="/login" className={buttonStyles({ variant: 'cta' })}>
              {t('links.login')}
            </Link>
          </div>
        ) : (
          <>
            <Typography variant="body" className="text-content-secondary mt-3 text-center">
              {t('resetPassword.intro')}
            </Typography>
            <form onSubmit={onSubmit} className="mt-6 grid gap-3">
              <div>
                <FieldLabel htmlFor="reset-new-password">
                  {t('resetPassword.newPassword')}
                </FieldLabel>
                <TextField
                  id="reset-new-password"
                  type="password"
                  value={newPassword}
                  onChange={(event) => setNewPassword(event.target.value)}
                  autoComplete="new-password"
                  minLength={8}
                  maxLength={128}
                  required
                  autoFocus
                  className="mt-1.5"
                />
                <Typography variant="meta" className="text-content-tertiary mt-1 block">
                  {t('resetPassword.passwordHint')}
                </Typography>
              </div>
              {reset.failure && (
                <Notice tone="danger" role="alert">
                  <AppFailureMessage failure={reset.failure} />
                </Notice>
              )}
              <Button type="submit" variant="cta" pending={reset.isPending}>
                {t('resetPassword.submit')}
              </Button>
            </form>
            {reset.failure?.reason === 'RESET_LINK_INVALID' && (
              <Link
                to="/forgot-password"
                className={typographyStyles({
                  variant: 'label',
                  className:
                    'text-link-fg hover:text-link-fg-hover mt-4 inline-flex min-h-11 w-full items-center justify-center underline',
                })}
              >
                {t('resetPassword.requestAgain')}
              </Link>
            )}
          </>
        )}
      </section>
    </main>
  )
}
