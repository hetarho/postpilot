import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useConfigurePublishingAgent, type PublishingAgent } from '@/entities/publishing-agent'
import { PublishVisibility } from '@/shared/api'
import { AppFailureMessage, Button, FieldLabel, Listbox, Notice, TextField } from '@/shared/ui'

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
  const update = useConfigurePublishingAgent(ownerId, agent.id)
  const failure = update.failure
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
        <FieldLabel id={`agent-category-label-${agent.id}`} htmlFor={`agent-category-${agent.id}`}>
          {t('configure.category')}
        </FieldLabel>
        <Listbox
          id={`agent-category-${agent.id}`}
          aria-labelledby={`agent-category-label-${agent.id}`}
          value={categoryId}
          options={agent.categories.map((category) => ({
            value: category.id,
            label: category.name,
          }))}
          onChange={setCategoryId}
          className="mt-2"
        />
      </div>
      <div>
        <FieldLabel
          id={`agent-visibility-label-${agent.id}`}
          htmlFor={`agent-visibility-${agent.id}`}
        >
          {t('configure.visibility')}
        </FieldLabel>
        {/* A `Listbox` rather than the `SegmentedControl` two fixed options would also allow (§7):
            it sits directly under 카테고리, and the pair reads as one settings form only while both
            wear the same field well. */}
        <Listbox<PublishVisibility>
          id={`agent-visibility-${agent.id}`}
          aria-labelledby={`agent-visibility-label-${agent.id}`}
          value={visibility}
          options={[
            { value: PublishVisibility.PUBLIC, label: t('visibility.public') },
            { value: PublishVisibility.PRIVATE, label: t('visibility.private') },
          ]}
          onChange={setVisibility}
          className="mt-2"
        />
      </div>
      <Button
        variant="secondary"
        onClick={() =>
          update.mutate({ label, defaultCategoryId: categoryId, defaultVisibility: visibility })
        }
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
