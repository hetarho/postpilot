import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useRevokePublishingAgent } from '@/entities/publishing-agent'
import { AppFailureMessage, Button, Dialog, Notice } from '@/shared/ui'

export function RevokePublishingAgent({
  ownerId,
  agentId,
  label,
}: {
  ownerId: string
  agentId: string
  label: string
}) {
  const { t } = useTranslation('publishing')
  const [confirming, setConfirming] = useState(false)
  const revoke = useRevokePublishingAgent(ownerId, agentId)
  const failure = revoke.failure
  return (
    <>
      <Button variant="danger" onClick={() => setConfirming(true)}>
        {t('revoke.action')}
      </Button>
      <Dialog
        open={confirming}
        title={t('revoke.title')}
        confirmLabel={t('revoke.action')}
        onClose={() => setConfirming(false)}
        onConfirm={() => revoke.mutate(undefined, { onSuccess: () => setConfirming(false) })}
        pending={revoke.isPending}
      >
        <div className="space-y-3">
          <p>{t('revoke.description', { label })}</p>
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
