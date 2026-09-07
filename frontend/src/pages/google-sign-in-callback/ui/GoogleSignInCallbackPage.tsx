import { useEffect, useRef, useState } from 'react'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { seedSessionCache } from '@/entities/session'
import { clearGoogleSignInAttempt, readGoogleSignInAttempt } from '@/features/sign-in-with-google'
import { appFailureFromConnect, AuthService, type AppFailure } from '@/shared/api'
import { SIGNED_IN_HOME, isInAppPath } from '@/shared/lib'
import { AppFailureMessage, Logo, Notice, Spinner, Typography, typographyStyles } from '@/shared/ui'

const callbackFailure: AppFailure = { reason: 'GOOGLE_SIGNIN_FAILED', params: {} }

export function GoogleSignInCallbackPage() {
  const { t } = useTranslation('auth')
  const search = useSearch({ from: '/login/google/callback' })
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const transport = useTransport()
  const mutation = useMutation(AuthService.method.signInWithGoogle)
  const started = useRef(false)
  const [callback] = useState(() => {
    const attempt = readGoogleSignInAttempt()
    return {
      attempt,
      invalid:
        Boolean(search.error) ||
        !search.code ||
        !search.state ||
        !attempt ||
        attempt.state !== search.state,
    }
  })
  const [failure, setFailure] = useState<AppFailure | undefined>(() =>
    callback.invalid ? callbackFailure : undefined,
  )

  useEffect(() => {
    if (started.current) return
    started.current = true
    if (callback.invalid || !callback.attempt || !search.code) {
      clearGoogleSignInAttempt()
      return
    }
    const attempt = callback.attempt

    mutation.mutate(
      {
        code: search.code,
        codeVerifier: attempt.verifier,
        redirectUri: `${window.location.origin}/login/google/callback`,
      },
      {
        onSuccess: (response) => {
          seedSessionCache(queryClient, transport, response)
          clearGoogleSignInAttempt()
          void navigate({
            to: isInAppPath(attempt.redirect) ? attempt.redirect : SIGNED_IN_HOME,
            replace: true,
          })
        },
        onError: (error) => {
          clearGoogleSignInAttempt()
          setFailure(appFailureFromConnect(error))
        },
      },
    )
  }, [callback, mutation, navigate, queryClient, search.code, transport])

  return (
    <main className="bg-surface-base text-content-primary flex min-h-full items-center justify-center px-4 py-10 sm:px-6">
      <section className="w-full max-w-xs text-center">
        <Typography variant="display" as="h1" className="flex flex-col items-center gap-4">
          <img src="/favicon.svg" alt="" className="size-16" />
          <Logo className="h-9" />
        </Typography>
        {failure ? (
          <>
            <Typography variant="title" as="h2" className="mt-6">
              {t('google.failed')}
            </Typography>
            <Notice tone="danger" role="alert" className="mt-4 text-left">
              <AppFailureMessage failure={failure} />
            </Notice>
            <Link
              to="/login"
              className={typographyStyles({
                variant: 'label',
                className:
                  'text-link-fg hover:text-link-fg-hover mt-5 inline-flex min-h-11 items-center px-2 underline',
              })}
            >
              {t('google.back')}
            </Link>
          </>
        ) : (
          <div role="status" className="mt-8 flex flex-col items-center gap-3">
            <Spinner />
            <Typography variant="body">{t('google.checking')}</Typography>
          </div>
        )}
      </section>
    </main>
  )
}
