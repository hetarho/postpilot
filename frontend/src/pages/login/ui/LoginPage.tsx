import { useState, type FormEvent } from 'react'
import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useLogin } from '@/entities/session'
import { SIGNED_IN_HOME, isInAppPath } from '@/shared/lib'
import {
  AppFailureMessage,
  Button,
  FieldLabel,
  Logo,
  TextField,
  Typography,
  typographyStyles,
} from '@/shared/ui'
import { InterfacePreferences } from '@/widgets/interface-preferences'

/** Credential refusals use one stable INVALID_CREDENTIALS reason. The server already refuses to
 *  distinguish an unknown id from a wrong password (spec/legacy/policy/auth.md), and structured failure
 *  parsing never falls back to backend prose that could reintroduce that enumeration signal. */
export function LoginPage() {
  const { t } = useTranslation(['auth', 'marketing'])
  const { redirect } = useSearch({ from: '/login' })
  const navigate = useNavigate()
  const login = useLogin()
  const [loginId, setLoginId] = useState('')
  const [password, setPassword] = useState('')
  const invalidCredentials = login.failure?.reason === 'INVALID_CREDENTIALS'

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    login.mutate(
      { loginId, password },
      {
        onSuccess: () => {
          // The guard put the blocked destination in `redirect`; anything else is not
          // ours to follow.
          void navigate({ to: isInAppPath(redirect) ? redirect : SIGNED_IN_HOME, replace: true })
        },
      },
    )
  }

  return (
    <main className="bg-surface-base text-content-primary relative flex min-h-full items-center justify-center px-4 py-10 sm:px-6">
      {/* Preferences are page chrome, not part of the credential form. Pinning them to the page
          edge keeps the same top-right location at every breakpoint and leaves the form itself
          truly centred. */}
      <div className="absolute top-4 right-4 z-10 sm:top-6 sm:right-6">
        <InterfacePreferences />
      </div>
      <div className="w-full max-w-xs">
        <form onSubmit={onSubmit} className="w-full">
          {/* The app icon is decorative beside the labelled wordmark, so the pair remains one
            concise heading for assistive technology. The compact phone lockup keeps the submit
            path above the software keyboard; wider screens give the mark its full presence. */}
          <Typography
            variant="display"
            as="h1"
            className="flex flex-col items-center gap-1 sm:gap-4"
          >
            <img src="/favicon.svg" alt="" className="h-10 w-10 sm:h-20 sm:w-20" />
            <Logo className="h-8 sm:h-9" />
          </Typography>
          <Typography variant="body" className="text-content-secondary mt-1 text-center">
            {t('login.intro', { ns: 'auth' })}
          </Typography>

          <FieldLabel htmlFor="loginId" className="mt-4 sm:mt-8">
            {t('login.id', { ns: 'auth' })}
          </FieldLabel>
          <TextField
            id="loginId"
            name="loginId"
            value={loginId}
            onChange={(event) => setLoginId(event.target.value)}
            autoComplete="username"
            // iOS defaults an <input type="text"> to `autocapitalize="sentences"`, so `hrlee`
            // is submitted as `Hrlee` and the server answers with the one generic failure it is
            // required to give — a silent loop the user cannot explain (design-language §7).
            autoCapitalize="none"
            autoCorrect="off"
            spellCheck={false}
            enterKeyHint="next"
            autoFocus
            required
            aria-invalid={invalidCredentials || undefined}
            aria-describedby={invalidCredentials ? 'login-error' : undefined}
            className="mt-1.5"
          />

          <FieldLabel htmlFor="password" className="mt-4">
            {t('login.password', { ns: 'auth' })}
          </FieldLabel>
          <TextField
            id="password"
            name="password"
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            autoComplete="current-password"
            // The return key is a real submit path here, so it says 이동 rather than 줄바꿈.
            enterKeyHint="go"
            required
            aria-invalid={invalidCredentials || undefined}
            aria-describedby={invalidCredentials ? 'login-error' : undefined}
            className="mt-1.5"
          />

          {/* Under the fields it describes and ABOVE the button, not after it (design-language §7,
            §4.3). Below the button it was both the lowest thing on the keyboard-covered screen and
            a ~32px insertion that shifted the whole form the instant the thumb lifted off 로그인. */}
          {login.failure && (
            <Typography
              variant="body"
              as="div"
              id="login-error"
              role="alert"
              className="text-field-error mt-3 break-words"
            >
              <AppFailureMessage failure={login.failure} />
            </Typography>
          )}

          {/* `pending`, not a label swap: 로그인 → 확인 중… resizes the target under the thumb that
            just pressed it (§6). */}
          <Button
            type="submit"
            variant="cta"
            pending={login.isPending}
            className="mt-4 w-full sm:mt-6"
          >
            {t('login.submit', { ns: 'auth' })}
          </Button>
        </form>
        {/* Below the credential action and OUTSIDE the form: a secondary link inside it would be
            one more tab stop between the password field and 로그인, and a link is not part of the
            submission. It changes nothing about the form's failure or redirect behavior. */}
        <p className="mt-6 text-center">
          <Link
            to="/about"
            // The blocked destination travels with the visitor: About hands it back to this page,
            // so reading the explanation mid-login does not reset where they were going.
            search={isInAppPath(redirect) ? { redirect } : {}}
            className={typographyStyles({
              variant: 'label',
              className:
                'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center px-2 underline',
            })}
          >
            {t('about.link', { ns: 'marketing' })}
          </Link>
        </p>
      </div>
    </main>
  )
}
