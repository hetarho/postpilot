import { useTranslation } from 'react-i18next'
import {
  useGuidelines,
  useSetDefaultGuidelineEnabled,
  type DefaultGuideline,
  type GuidelineKind,
} from '@/entities/guideline'
import {
  Button,
  ChoiceRow,
  FieldMessage,
  ListGroup,
  LoadingState,
  Notice,
  Typography,
} from '@/shared/ui'

/** A kind's 기본 지침 as rows that are on or off, in the product's order and without the 원칙
 *  (GUIDE-41, GUIDE-51): a press puts one in use or out of it and saves at once (GUIDE-43), so
 *  several are switched in one visit. The list is the screen's own query, so the guidelines screen
 *  and anything counting what is in use follow the same write. */
export function DefaultGuidelineChoices({
  ownerId,
  kind,
  label,
}: {
  ownerId: string
  kind: GuidelineKind
  /** The list's accessible name. */
  label: string
}) {
  const { t } = useTranslation(['guidelines', 'common'])
  const { defaults, isPending, isError, isFetching, refetch } = useGuidelines(ownerId, kind)
  const choices = defaults.filter((guideline) => !guideline.principle)
  if (isError && defaults.length === 0)
    return (
      <Notice tone="danger" role="alert">
        <span>{t('defaults.loadFailed')}</span>
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
  if (choices.length === 0)
    return (
      <Typography variant="body" as="p" role="status" className="text-content-secondary">
        {t('defaults.none')}
      </Typography>
    )
  return (
    <div role="group" aria-label={label}>
      <ListGroup>
        {choices.map((guideline, index) => (
          <DefaultGuidelineChoice
            key={guideline.key}
            ownerId={ownerId}
            kind={kind}
            guideline={guideline}
            first={index === 0}
          />
        ))}
      </ListGroup>
    </div>
  )
}

/** One 기본 지침. Its own mutation, so the row being saved shows it and a refused switch is said
 *  under the row it was refused for; the optimistic write is undone, so the row reads as it was. */
function DefaultGuidelineChoice({
  ownerId,
  kind,
  guideline,
  first,
}: {
  ownerId: string
  kind: GuidelineKind
  guideline: DefaultGuideline
  first: boolean
}) {
  const toggle = useSetDefaultGuidelineEnabled(ownerId, kind)
  return (
    <li data-autofocus={first || undefined}>
      <ChoiceRow
        mode="checkbox"
        checked={guideline.enabled}
        name={guideline.name}
        pending={toggle.isPending}
        // The refusal renders under the row from the mutation's own error.
        onPick={() => void toggle.setEnabled(guideline.key, !guideline.enabled).catch(() => {})}
      />
      {toggle.errorMessage && (
        <FieldMessage className="px-4 pb-3">{toggle.errorMessage}</FieldMessage>
      )}
    </li>
  )
}
