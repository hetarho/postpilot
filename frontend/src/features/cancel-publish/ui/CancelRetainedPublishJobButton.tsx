import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useCancelRetainedPublishJob } from '@/entities/publish-job'
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
  const cancel = useCancelRetainedPublishJob(ownerId, jobId)
  const failure = cancel.failure

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
        onConfirm={() => cancel.mutate(undefined, { onSuccess: () => setConfirming(false) })}
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
