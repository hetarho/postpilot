import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useRouterState, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { LoginForm } from '@/features/log-in'
import { GoogleSignInButton } from '@/features/sign-in-with-google'
import { SIGNED_IN_HOME, isInAppPath } from '@/shared/lib'
import { Notice, Typography, typographyStyles } from '@/shared/ui'
import { InterfacePreferences } from '@/widgets/interface-preferences'

export function LoginPage() {
  const { t } = useTranslation(['auth', 'marketing'])
  const { redirect } = useSearch({ from: '/login' })
  const navigate = useNavigate()
  const initialNotice = useRouterState({ select: (state) => state.location.state.notice })
  const [passwordChanged] = useState(initialNotice === 'password-changed')
  const noticeCleared = useRef(false)
  const [rememberMe, setRememberMe] = useState(false)

  useEffect(() => {
    if (!passwordChanged || noticeCleared.current) return
    noticeCleared.current = true
    void navigate({
      to: '/login',
      search: isInAppPath(redirect) ? { redirect } : {},
      replace: true,
      state: (previous) => ({ ...previous, notice: undefined }),
    })
  }, [navigate, passwordChanged, redirect])

  return (
    <main className="bg-surface-base text-content-primary relative flex min-h-full items-center justify-center px-4 py-10 sm:px-6">
      {/* Preferences are page chrome, not part of the credential form. Pinning them to the page
          edge keeps the same top-right location at every breakpoint and leaves the form itself
          truly centred. */}
      <div className="absolute top-4 right-4 z-10 sm:top-6 sm:right-6">
        <InterfacePreferences />
      </div>
      <div className="w-full max-w-xs">
        {passwordChanged && (
          <Notice tone="info" role="status" className="mb-4">
            {t('login.passwordChanged', { ns: 'auth' })}
          </Notice>
        )}
        <LoginForm
          rememberMe={rememberMe}
          onRememberMeChange={setRememberMe}
          onSuccess={() => {
            void navigate({ to: isInAppPath(redirect) ? redirect : SIGNED_IN_HOME, replace: true })
          }}
        />
        <GoogleSignInButton redirect={redirect} rememberMe={rememberMe} />
        {/* Below the credential action and OUTSIDE the form: a secondary link inside it would be
            one more tab stop between the password field and 로그인, and a link is not part of the
            submission. It changes nothing about the form's failure or redirect behavior. The way
            to signup is a question with its answer, not a bare label beside 비밀번호 찾기 — the
            sentence is what a visitor on the wrong screen reads first (AUTH-42). The link keeps
            the bare verb as its accessible name. */}
        <nav
          className="mt-6 flex flex-col items-center gap-2"
          aria-label={t('links.more', { ns: 'auth' })}
        >
          <Typography variant="label" as="p" className="text-content-secondary">
            {t('links.noAccount', { ns: 'auth' })}{' '}
            <Link
              to="/signup"
              search={isInAppPath(redirect) ? { redirect } : {}}
              className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center px-1 underline"
            >
              {t('links.signup', { ns: 'auth' })}
            </Link>
          </Typography>
          <div className="flex flex-wrap items-center justify-center gap-3">
            <Link
              to="/forgot-password"
              search={isInAppPath(redirect) ? { redirect } : {}}
              className={typographyStyles({
                variant: 'label',
                className:
                  'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center px-2 underline',
              })}
            >
              {t('links.forgotPassword', { ns: 'auth' })}
            </Link>
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
          </div>
        </nav>
      </div>
    </main>
  )
}
