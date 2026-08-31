import { useTranslation } from 'react-i18next'
import type { AppFailure } from '@/shared/api'
import { AppFailureMessage, Button, Notice } from '@/shared/ui'

export function FailureNotice({
  failure,
  message,
  onRetry,
}: {
  failure?: AppFailure
  message?: string
  onRetry?: () => void
}) {
  const { t } = useTranslation(['posts', 'common'])
  return (
    <Notice tone="danger" role="alert">
      {failure ? (
        <AppFailureMessage failure={failure} />
      ) : (
        <span className="min-w-0 break-words">
          {message || t('generation.failed', { ns: 'posts' })}
        </span>
      )}
      {onRetry && (
        <Button
          variant="ghost"
          onClick={onRetry}
          className="text-notice-danger-fg shrink-0 underline"
        >
          {t('action.retry', { ns: 'common' })}
        </Button>
      )}
    </Notice>
  )
}
