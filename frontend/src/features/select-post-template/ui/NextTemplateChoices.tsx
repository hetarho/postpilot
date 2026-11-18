import { useTranslation } from 'react-i18next'
import {
  NO_TEMPLATE_VALUE,
  listedLastTemplate,
  noTemplateLabel,
  rememberLastTemplate,
  useLastTemplate,
  useTemplates,
} from '@/entities/template'
import { Button, ChoiceRow, ListGroup, LoadingState, Notice } from '@/shared/ui'

/** The template the next new post starts on, as one pick: 없음 and the account's templates, the
 *  one last specified checked — 없음 until one has been, and again once that one is deleted.
 *  Choosing one remembers it for the account on this browser (`lastTemplate`), the same memory
 *  the post guide's 템플릿 step writes, and `onChosen` runs. */
export function NextTemplateChoices({
  ownerId,
  label,
  onChosen,
}: {
  ownerId: string
  /** The list's accessible name. */
  label: string
  onChosen?: () => void
}) {
  const { t } = useTranslation(['templates', 'common'])
  const { templates, isPending, isError, isFetching, refetch } = useTemplates(ownerId)
  const current = listedLastTemplate(useLastTemplate(ownerId), templates) ?? NO_TEMPLATE_VALUE
  if (isError && templates.length === 0)
    return (
      <Notice tone="danger" role="alert">
        <span>{t('loadFailed')}</span>
        <Button
          variant="ghost"
          onClick={refetch}
          pending={isFetching}
          className="text-notice-danger-fg underline"
        >
          {t('action.retry', { ns: 'common' })}
        </Button>
      </Notice>
    )
  if (isPending) return <LoadingState variant="list" label={t('state.loading', { ns: 'common' })} />
  const choices = [
    { id: NO_TEMPLATE_VALUE, name: noTemplateLabel() },
    ...templates.map(({ id, name }) => ({ id, name })),
  ]
  return (
    <div role="radiogroup" aria-label={label}>
      <ListGroup>
        {choices.map((choice) => (
          <li key={choice.id} data-autofocus={choice.id === current || undefined}>
            <ChoiceRow
              mode="radio"
              checked={choice.id === current}
              name={choice.name}
              onPick={() => {
                rememberLastTemplate(ownerId, choice.id)
                onChosen?.()
              }}
            />
          </li>
        ))}
      </ListGroup>
    </div>
  )
}
