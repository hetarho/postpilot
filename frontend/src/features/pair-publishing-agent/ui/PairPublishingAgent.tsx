import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { publishingAgentsQueryKey } from '@/entities/publishing-agent'
import { appFailureFromConnect, publishingClientFor } from '@/shared/api'
import { formatDateTime } from '@/shared/lib'
import { AppFailureMessage, Button, FieldLabel, Notice, TextField, Typography } from '@/shared/ui'

export function PairPublishingAgent({ ownerId }: { ownerId: string }) {
  const { t } = useTranslation('publishing')
  const [label, setLabel] = useState<string>(() => t('pair.defaultLabel'))
  const transport = useTransport()
  const client = useMemo(() => publishingClientFor(transport), [transport])
  const queryClient = useQueryClient()
  const pairing = useMutation({
    mutationFn: () => client.createAgentPairing({ label: label.trim() }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: publishingAgentsQueryKey(ownerId) }),
  })
  const failure = pairing.error ? appFailureFromConnect(pairing.error) : undefined

  return (
    <section aria-labelledby="pair-agent-heading">
      <Typography variant="title" id="pair-agent-heading">
        {t('pair.title')}
      </Typography>
      <Typography variant="body" className="text-content-secondary mt-2">
        {t('pair.description')}
      </Typography>
      <div className="mt-4">
        <FieldLabel htmlFor="publishing-agent-label">{t('pair.label')}</FieldLabel>
        <TextField
          id="publishing-agent-label"
          value={label}
          onChange={(event) => setLabel(event.target.value)}
          type="text"
          autoComplete="off"
          autoCapitalize="off"
          autoCorrect="off"
          enterKeyHint="done"
          className="mt-2"
        />
      </div>
      <Button
        variant="cta"
        className="mt-4 w-full sm:w-auto"
        onClick={() => pairing.mutate()}
        pending={pairing.isPending}
        disabled={!label.trim()}
      >
        {t('pair.create')}
      </Button>
      <div className="mt-4 empty:hidden">
        {pairing.data && (
          <Notice tone="success" role="status">
            <span>
              {t('pair.code')}{' '}
              <Typography variant="body" as="span" mono>
                {pairing.data.deviceCode}
              </Typography>
              <br />
              {t('pair.expires', { date: formatDateTime(pairing.data.expiresAt) })}
            </span>
          </Notice>
        )}
        {failure && (
          <Notice tone="danger" role="alert">
            <AppFailureMessage failure={failure} />
          </Notice>
        )}
      </div>
    </section>
  )
}
