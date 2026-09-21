import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Typography } from '@/shared/ui'

export function ModelResultsState({
  isPending,
  isError,
  onRetry,
  children,
}: {
  isPending: boolean
  isError: boolean
  onRetry: () => void
  children: ReactNode
}) {
  const { t } = useTranslation(['models', 'common'])
  if (isPending)
    return (
      <Typography variant="body" role="status" className="text-content-tertiary mt-4">
        {t('state.loading', { ns: 'common' })}
      </Typography>
    )
  if (isError)
    return (
      <div className="mt-4">
        <Typography variant="body" role="alert">
          {t('page.resultsFailed', { ns: 'models' })}
        </Typography>
        <Button variant="ghost" className="mt-2" onClick={onRetry}>
          {t('action.retry', { ns: 'common' })}
        </Button>
      </div>
    )
  return children
}
