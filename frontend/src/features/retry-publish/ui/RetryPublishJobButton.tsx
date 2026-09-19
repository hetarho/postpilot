import { useTranslation } from 'react-i18next'
import { useRetryPublishJob } from '@/entities/publish-job'
import { AppFailureMessage, Button, Notice } from '@/shared/ui'

interface RetryPublishJobButtonProps {
  ownerId: string
  jobId: string
}

export function RetryPublishJobButton({ ownerId, jobId }: RetryPublishJobButtonProps) {
  const { t } = useTranslation('publishing')
  const retry = useRetryPublishJob(ownerId, jobId)
  const failure = retry.failure

  return (
    <div>
      <Button variant="secondary" disabled={retry.isPending} onClick={() => retry.mutate()}>
        {retry.isPending ? t('retry.pending') : t('retry.action')}
      </Button>
      {failure && (
        <Notice tone="danger" role="alert" className="mt-2">
          <AppFailureMessage failure={failure} />
        </Notice>
      )}
    </div>
  )
}
