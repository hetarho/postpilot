import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { publishingAgentsQueryKey } from '@/entities/publishing-agent'
import { appFailureFromConnect, publishingClientFor } from '@/shared/api'
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
  const transport = useTransport()
  const client = useMemo(() => publishingClientFor(transport), [transport])
  const queryClient = useQueryClient()
  const revoke = useMutation({
    mutationFn: () => client.revokePublishingAgent({ agentId }),
    onSuccess: async () => {
      setConfirming(false)
      await queryClient.invalidateQueries({
        queryKey: publishingAgentsQueryKey(ownerId),
      })
    },
  })
  const failure = revoke.error ? appFailureFromConnect(revoke.error) : undefined
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
        onConfirm={() => revoke.mutate()}
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
