import i18next from 'i18next'
import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import {
  type CatalogModel,
  type StageName,
  refKey,
  useSaveSelection,
  useStageSelection,
} from '@/entities/model-catalog'
import {
  AppFailureMessage,
  FieldLabel,
  FieldMessage,
  Listbox,
  Typography,
  type ListboxOption,
} from '@/shared/ui'

/** The per-stage model dropdown (PRD §3.3, §6.4, F-4).
 *
 *  Lists what the registry offers for the stage — vision models only for `observe` —
 *  with a disabled model greyed and its reason shown, a vanished saved choice greyed
 *  and its reason given under the field, and no pre-selection: until the user picks, the stage is
 *  empty and `useStageSelection(stage).selected` is null, which is what the generation and
 *  analysis actions block on ([I3]). Pages mount it; features never import it. */
export function StageModelSelect({
  stage,
  className,
  optional = false,
}: {
  stage: StageName
  className?: string
  optional?: boolean
}) {
  const { t } = useTranslation('models')
  const id = useId()
  const { models, selected, unavailable, isPending, isError } = useStageSelection(stage)
  const save = useSaveSelection()

  // The saved choice's key when it can be shown as chosen; the greyed unusable entry
  // otherwise. An empty value is the placeholder.
  const value = selected ? refKey(selected) : unavailable ? UNAVAILABLE_VALUE : ''
  const labelId = `${id}-label`
  const loadErrorId = `${id}-load-error`
  const saveErrorId = `${id}-save-error`
  const unavailableId = `${id}-unavailable`
  const describedBy = [
    isError && loadErrorId,
    save.failure && saveErrorId,
    unavailable && unavailableId,
  ]
    .filter(Boolean)
    .join(' ')

  const options: ListboxOption<string>[] = [
    { value: '', label: t('select') },
    // An option's text is the CHOICE, not the explanation (§7). This entry is the field's current
    // value, so its whole string has to fit the CLOSED trigger — ~284px at 360px, which a
    // `provider/model` path alone already fills, and the trigger truncates. The reason therefore
    // goes in the message slot under the field, where it cannot be cut off.
    ...(unavailable
      ? [{ value: UNAVAILABLE_VALUE, label: refKey(unavailable.ref), disabled: true }]
      : []),
    ...models.map((model) => ({
      value: refKey(model.ref),
      label: optionLabel(model),
      disabled: model.disabled || !model.affordable,
    })),
  ]

  return (
    <div className={className}>
      <FieldLabel id={labelId} htmlFor={id}>
        {t('selectField.label', {
          stage: t(`selectField.stage.${stage}`),
          optional: optional ? t('selectField.optional') : '',
        })}
      </FieldLabel>
      <Listbox
        id={id}
        aria-labelledby={labelId}
        value={value}
        options={options}
        disabled={isPending || save.isPending}
        aria-invalid={isError || Boolean(save.failure) || undefined}
        aria-describedby={describedBy || undefined}
        onChange={(next) => {
          const chosen = models.find((model) => refKey(model.ref) === next)
          if (chosen && !chosen.disabled && chosen.affordable) save.save(stage, chosen.ref)
        }}
        className="mt-1"
      />
      {/* Visible, not sr-only: the control greys out for the 1–3s a SaveSelection takes on mobile
          data, and a touch user watching the field it just closed over is exactly who needs the
          cause (§6). The region stays mounted so it announces when it fills, and `empty:hidden`
          keeps it out of the layout while it is idle. */}
      <Typography
        variant="body"
        as="p"
        role="status"
        className="text-content-tertiary mt-1 empty:hidden"
      >
        {save.isPending ? t('selectField.saving') : null}
      </Typography>
      {unavailable && (
        // `status`, not the default `alert`: this is a standing condition of the saved value, not
        // something that just went wrong, and it renders on first paint.
        <FieldMessage id={unavailableId} role="status" className="mt-1">
          {unavailable.reason}
        </FieldMessage>
      )}
      {isError && (
        <FieldMessage id={loadErrorId} className="mt-1">
          {t('selectField.loadFailed')}
        </FieldMessage>
      )}
      {save.failure && (
        <Typography
          variant="body"
          as="div"
          id={saveErrorId}
          role="alert"
          className="text-field-error mt-1 break-words"
        >
          <AppFailureMessage failure={save.failure} />
        </Typography>
      )}
    </div>
  )
}

const UNAVAILABLE_VALUE = '__unavailable__'

/** `<label> 👁 구조화 응답` badges for what the model can do (PRD §6.4), and the reason when it
 *  cannot be chosen. Plain text: a listbox row renders the label as one string.
 *
 *  These strings are only ever read inside the OPEN panel, where the row wraps them — a disabled
 *  option can never become the closed trigger's value, because `onChange` refuses it and an
 *  unusable saved choice is rendered as the separate entry above. */
function optionLabel(model: CatalogModel): string {
  const badges = [
    model.vision && '👁',
    model.structuredOutput && i18next.t('selectField.structuredOutput', { ns: 'models' }),
  ]
    .filter(Boolean)
    .join(' ')
  // A locked model stays listed rather than vanishing, and says which tier unlocks it: the
  // reason it cannot be chosen is the only thing this entry has to teach. A provider without
  // a key is the more immediate obstacle, so that reason wins when both apply.
  const reason = model.disabled
    ? ` (${model.disabledReason})`
    : !model.affordable
      ? ` (${i18next.t('selectField.unaffordable', { ns: 'models', credits: model.requiredCredits })})`
      : ''
  return `${model.label}${badges ? ` ${badges}` : ''}${reason}`
}
