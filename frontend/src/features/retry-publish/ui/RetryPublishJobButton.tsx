import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { retryablePublishJobsQueryKey } from '@/entities/publish-job'
import { appFailureFromConnect, publishingClientFor } from '@/shared/api'
import { AppFailureMessage, Button, Notice } from '@/shared/ui'

interface RetryPublishJobButtonProps {
  ownerId: string
  jobId: string
}

export function RetryPublishJobButton({ ownerId, jobId }: RetryPublishJobButtonProps) {
  const { t } = useTranslation('publishing')
  const transport = useTransport()
  const client = useMemo(() => publishingClientFor(transport), [transport])
  const queryClient = useQueryClient()
  const retry = useMutation({
    mutationFn: async () => client.retryPublish({ jobId }),
    onSuccess: () =>
      queryClient.invalidateQueries({
        queryKey: retryablePublishJobsQueryKey(ownerId),
      }),
  })
  const failure = retry.error ? appFailureFromConnect(retry.error) : undefined

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
