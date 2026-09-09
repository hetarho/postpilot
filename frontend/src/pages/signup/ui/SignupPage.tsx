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
  const [passwordConfirm, setPasswordConfirm] = useState('')
  const [mismatch, setMismatch] = useState(false)
  const [mailed, setMailed] = useState(false)

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    // Checked here, not with a native constraint: the message has to be i18next copy, and a
    // browser validation bubble cannot be localized. A mismatch never reaches the server.
    if (password !== passwordConfirm) {
      setMismatch(true)
      return
    }
    signup.mutate({ email, password }, { onSuccess: () => setMailed(true) })
  }

  return (
    <main className="bg-surface-base text-content-primary relative flex min-h-full items-center justify-center px-4 py-10 sm:px-6">
      <div className="absolute top-4 right-4 z-10 sm:top-6 sm:right-6">
        <InterfacePreferences />
      </div>
      <section className="w-full max-w-xs" aria-labelledby="signup-heading">
        {/* The lockup is brand, not the heading: login and signup share it, and the word under it
            is what tells the two screens apart (AUTH-42). */}
        <div className="flex flex-col items-center gap-1 sm:gap-4">
          <img src="/favicon.svg" alt="" className="h-10 w-10 sm:h-20 sm:w-20" />
          <Logo className="h-8 sm:h-9" />
        </div>
        <Typography
          variant="display"
          as="h1"
          id="signup-heading"
          className="mt-4 text-center sm:mt-6"
        >
          {t('signup.heading', { ns: 'auth' })}
        </Typography>
        {mailed ? (
          <div className="mt-6 grid gap-4 text-center">
            {/* Still on the sign-up path, so the page keeps its title and the mailed state is a
                section under it — unlike forgot-password, whose sent state replaces the heading. */}
            <Typography variant="title" as="h2">
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
          </div>
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
                onChange={(event) => {
                  setPassword(event.target.value)
                  setMismatch(false)
                }}
                autoComplete="new-password"
                minLength={8}
                maxLength={128}
                required
                aria-invalid={mismatch || undefined}
                aria-describedby={mismatch ? 'signup-password-mismatch' : undefined}
                className="mt-1.5"
              />
              <Typography variant="meta" className="text-content-tertiary mt-1 block">
                {t('signup.passwordHint', { ns: 'auth' })}
              </Typography>
              <FieldLabel htmlFor="signup-password-confirm" className="mt-4">
                {t('field.passwordConfirm', { ns: 'auth' })}
              </FieldLabel>
              <TextField
                id="signup-password-confirm"
                name="passwordConfirm"
                type="password"
                value={passwordConfirm}
                onChange={(event) => {
                  setPasswordConfirm(event.target.value)
                  setMismatch(false)
                }}
                autoComplete="new-password"
                minLength={8}
                maxLength={128}
                required
                aria-invalid={mismatch || undefined}
                aria-describedby={mismatch ? 'signup-password-mismatch' : undefined}
                className="mt-1.5"
              />
              {/* Under the fields it describes, the way the login form places its own error;
                  `Notice` carries no id, and both password fields point here. */}
              {mismatch && (
                <Typography
                  variant="body"
                  as="div"
                  id="signup-password-mismatch"
                  role="alert"
                  className="text-field-error mt-3 break-words"
                >
                  {t('signup.passwordMismatch', { ns: 'auth' })}
                </Typography>
              )}
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
            {/* The way back to login is a question with its answer (AUTH-42); the link keeps the
                bare verb as its accessible name. */}
            <nav
              className="mt-6 flex flex-col items-center gap-2"
              aria-label={t('links.more', { ns: 'auth' })}
            >
              <Typography variant="label" as="p" className="text-content-secondary">
                {t('links.haveAccount', { ns: 'auth' })}{' '}
                <Link
                  to="/login"
                  search={redirect ? { redirect } : {}}
                  className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center px-1 underline"
                >
                  {t('links.login', { ns: 'auth' })}
                </Link>
              </Typography>
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
      </section>
    </main>
  )
}
