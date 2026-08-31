import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { publishingAgentsQueryKey, type PublishingAgent } from '@/entities/publishing-agent'
import { appFailureFromConnect, publishingClientFor, PublishVisibility } from '@/shared/api'
import { AppFailureMessage, Button, FieldLabel, Notice, Select, TextField } from '@/shared/ui'

export function ConfigurePublishingAgent({
  ownerId,
  agent,
}: {
  ownerId: string
  agent: PublishingAgent
}) {
  const { t } = useTranslation('publishing')
  const [label, setLabel] = useState(agent.label)
  const [categoryId, setCategoryId] = useState(agent.defaultCategoryId)
  const [visibility, setVisibility] = useState(agent.defaultVisibility)
  const transport = useTransport()
  const client = useMemo(() => publishingClientFor(transport), [transport])
  const queryClient = useQueryClient()
  const update = useMutation({
    mutationFn: () =>
      client.updatePublishingAgent({
        agentId: agent.id,
        label: label.trim(),
        defaultCategoryId: categoryId,
        defaultVisibility: visibility,
      }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: publishingAgentsQueryKey(ownerId) }),
  })
  const failure = update.error ? appFailureFromConnect(update.error) : undefined
  if (!agent.ready) return null
  return (
    <div className="mt-4 grid gap-4">
      <div>
        <FieldLabel htmlFor={`agent-label-${agent.id}`}>{t('configure.label')}</FieldLabel>
        <TextField
          id={`agent-label-${agent.id}`}
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
      <div>
        <FieldLabel htmlFor={`agent-category-${agent.id}`}>{t('configure.category')}</FieldLabel>
        <Select
          id={`agent-category-${agent.id}`}
          value={categoryId}
          onChange={(event) => setCategoryId(event.target.value)}
          className="mt-2"
        >
          {agent.categories.map((category) => (
            <option key={category.id} value={category.id}>
              {category.name}
            </option>
          ))}
        </Select>
      </div>
      <div>
        <FieldLabel htmlFor={`agent-visibility-${agent.id}`}>
          {t('configure.visibility')}
        </FieldLabel>
        <Select
          id={`agent-visibility-${agent.id}`}
          value={visibility}
          onChange={(event) => setVisibility(Number(event.target.value) as PublishVisibility)}
          className="mt-2"
        >
          <option value={PublishVisibility.PUBLIC}>{t('visibility.public')}</option>
          <option value={PublishVisibility.PRIVATE}>{t('visibility.private')}</option>
        </Select>
      </div>
      <Button
        variant="secondary"
        onClick={() => update.mutate()}
        pending={update.isPending}
        disabled={!label.trim() || !categoryId}
      >
        {t('configure.save')}
      </Button>
      {failure && (
        <Notice tone="danger" role="alert">
          <AppFailureMessage failure={failure} />
        </Notice>
      )}
    </div>
  )
}
