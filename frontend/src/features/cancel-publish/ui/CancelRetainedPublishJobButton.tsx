import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { retryablePublishJobsQueryKey } from '@/entities/publish-job'
import { appFailureFromConnect, publishingClientFor } from '@/shared/api'
import { AppFailureMessage, Button, Dialog, Notice } from '@/shared/ui'

interface CancelRetainedPublishJobButtonProps {
  ownerId: string
  jobId: string
  postSlug: string
}

export function CancelRetainedPublishJobButton({
  ownerId,
  jobId,
  postSlug,
}: CancelRetainedPublishJobButtonProps) {
  const { t } = useTranslation('publishing')
  const [confirming, setConfirming] = useState(false)
  const transport = useTransport()
  const client = useMemo(() => publishingClientFor(transport), [transport])
  const queryClient = useQueryClient()
  const cancel = useMutation({
    mutationFn: () => client.cancelPublish({ jobId }),
    onSuccess: async () => {
      setConfirming(false)
      await queryClient.invalidateQueries({
        queryKey: retryablePublishJobsQueryKey(ownerId),
      })
    },
  })
  const failure = cancel.error ? appFailureFromConnect(cancel.error) : undefined

  return (
    <>
      <Button variant="danger" onClick={() => setConfirming(true)}>
        {t('cancelRetained.action')}
      </Button>
      <Dialog
        open={confirming}
        title={t('cancelRetained.title')}
        confirmLabel={t('cancelRetained.confirm')}
        onClose={() => setConfirming(false)}
        onConfirm={() => cancel.mutate()}
        pending={cancel.isPending}
      >
        <div className="space-y-3">
          <p>{t('cancelRetained.description', { postSlug })}</p>
          {failure && (
            <Notice tone="danger" role="alert">
              <AppFailureMessage failure={failure} />
            </Notice>
          )}
        </div>
      </Dialog>
    </>
  )
}
