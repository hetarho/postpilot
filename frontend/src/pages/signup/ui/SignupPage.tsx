import { useState, type FormEvent } from 'react'
import { Link, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { GoogleSignInButton } from '@/features/sign-in-with-google'
import { useSignUp } from '@/features/sign-up'
import { useResendVerification } from '@/features/verify-email'
import {
  AppFailureMessage,
  Button,
  FieldLabel,
  Logo,
  Notice,
  TextField,
  Typography,
  typographyStyles,
} from '@/shared/ui'
import { InterfacePreferences } from '@/widgets/interface-preferences'

export function SignupPage() {
  const { t } = useTranslation(['auth', 'marketing'])
  const { redirect } = useSearch({ from: '/signup' })
  const signup = useSignUp()
  const resend = useResendVerification()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [mailed, setMailed] = useState(false)

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    signup.mutate({ email, password }, { onSuccess: () => setMailed(true) })
  }

  return (
    <main className="bg-surface-base text-content-primary relative flex min-h-full items-center justify-center px-4 py-10 sm:px-6">
      <div className="absolute top-4 right-4 z-10 sm:top-6 sm:right-6">
        <InterfacePreferences />
      </div>
      <div className="w-full max-w-xs">
        <Typography variant="display" as="h1" className="flex flex-col items-center gap-1 sm:gap-4">
          <img src="/favicon.svg" alt="" className="h-10 w-10 sm:h-20 sm:w-20" />
          <Logo className="h-8 sm:h-9" />
        </Typography>
        {mailed ? (
          <section className="mt-6 grid gap-4 text-center" aria-labelledby="signup-mailed">
            <Typography variant="title" as="h2" id="signup-mailed">
              {t('signup.mailedHeading', { ns: 'auth' })}
            </Typography>
            <Typography variant="body" className="text-content-secondary break-words">
              {t('signup.mailedBody', { ns: 'auth', email })}
            </Typography>
            {resend.failure && (
              <Notice tone="danger" role="alert">
                <AppFailureMessage failure={resend.failure} />
              </Notice>
            )}
            <Button
              variant="secondary"
              pending={resend.isPending}
              onClick={() => resend.mutate({ email })}
            >
              {t('verification.resend', { ns: 'auth' })}
            </Button>
            <Link
              to="/login"
              search={redirect ? { redirect } : {}}
              className={typographyStyles({
                variant: 'label',
                className:
                  'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center justify-center underline',
              })}
            >
              {t('links.login', { ns: 'auth' })}
            </Link>
          </section>
        ) : (
          <>
            <Typography variant="body" className="text-content-secondary mt-1 text-center">
              {t('signup.intro', { ns: 'auth' })}
            </Typography>
            <form onSubmit={onSubmit} className="mt-6 w-full">
              <FieldLabel htmlFor="signup-email">{t('field.email', { ns: 'auth' })}</FieldLabel>
              <TextField
                id="signup-email"
                name="email"
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
              <FieldLabel htmlFor="signup-password" className="mt-4">
                {t('field.password', { ns: 'auth' })}
              </FieldLabel>
              <TextField
                id="signup-password"
                name="password"
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                autoComplete="new-password"
                minLength={8}
                maxLength={128}
                required
                className="mt-1.5"
              />
              <Typography variant="meta" className="text-content-tertiary mt-1 block">
                {t('signup.passwordHint', { ns: 'auth' })}
              </Typography>
              {signup.failure && (
                <Notice tone="danger" role="alert" className="mt-3">
                  <AppFailureMessage failure={signup.failure} />
                </Notice>
              )}
              <Button
                type="submit"
                variant="cta"
                pending={signup.isPending}
                className="mt-4 w-full"
              >
                {t('signup.submit', { ns: 'auth' })}
              </Button>
            </form>
            <GoogleSignInButton redirect={redirect} />
            <nav
              className="mt-6 flex items-center justify-center gap-3"
              aria-label={t('links.more', { ns: 'auth' })}
            >
              <Link
                to="/login"
                search={redirect ? { redirect } : {}}
                className={typographyStyles({
                  variant: 'label',
                  className: 'text-link-fg min-h-11 px-2 py-3 underline',
                })}
              >
                {t('links.login', { ns: 'auth' })}
              </Link>
              <Link
                to="/about"
                search={redirect ? { redirect } : {}}
                className={typographyStyles({
                  variant: 'label',
                  className: 'text-link-fg min-h-11 px-2 py-3 underline',
                })}
              >
                {t('about.link', { ns: 'marketing' })}
              </Link>
            </nav>
          </>
        )}
      </div>
    </main>
  )
}
