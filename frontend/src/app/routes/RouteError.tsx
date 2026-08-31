import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useRouter, type ErrorComponentProps } from '@tanstack/react-router'
import { appFailureFromConnect } from '@/shared/api'
import { AppFailureMessage, Button, Notice } from '@/shared/ui'

/** The last-resort route boundary deliberately renders only the public failure contract.
 * TanStack's default boundary exposes `error.message`, which may contain private backend prose. */
export function RouteError({ error, reset }: ErrorComponentProps) {
  const { t } = useTranslation('common')
  const router = useRouter()
  const [retrying, setRetrying] = useState(false)
  const failure = appFailureFromConnect(error)

  const retry = async () => {
    setRetrying(true)
    try {
      await router.invalidate()
    } finally {
      // A failed invalidation installs a fresh error match; a successful one unmounts this
      // component. Resetting last handles both without ever rendering the captured exception.
      reset()
      setRetrying(false)
    }
  }

  return (
    <main className="mx-auto flex min-h-dvh w-full max-w-xl flex-col justify-center gap-4 px-4 py-8">
      <Notice tone="danger" role="alert">
        <AppFailureMessage failure={failure} />
      </Notice>
      <div>
        <Button pending={retrying} onClick={() => void retry()}>
          {t('action.retry')}
        </Button>
      </div>
    </main>
  )
}
