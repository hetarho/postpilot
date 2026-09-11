import i18next from 'i18next'
import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import {
  type CatalogModel,
  type ModelAvailability,
  type StageName,
  levelPrefix,
  refKey,
  useSaveSelection,
  useStageSelection,
  verdictOf,
} from '@/entities/model-catalog'
import {
  AppFailureMessage,
  Button,
  FieldLabel,
  FieldMessage,
  Listbox,
  Typography,
  type ListboxOption,
} from '@/shared/ui'

/** The per-stage model dropdown (PRD §3.3, §6.4, F-4).
 *
 *  Lists the models registered to the stage's purpose (change 20; observe's vision
 *  requirement is enforced at registration), with a disabled model greyed and its reason
 *  shown, a vanished saved choice greyed
 *  and its reason given under the field, and no pre-selection: until the user picks, the stage is
 *  empty and `useStageSelection(stage).selected` is null, which is what the generation and
 *  analysis actions block on ([I3]). Pages mount it; features never import it. */
export function StageModelSelect({
  stage,
  className,
  optional = false,
  disabled = false,
  requireVideoInput = false,
  availability,
}: {
  stage: StageName
  className?: string
  optional?: boolean
  disabled?: boolean
  requireVideoInput?: boolean
  /** A page's own per-model verdict for its workflow (T112). Absent, the picker behaves as
   *  it always has; present, a model the verdict refuses is greyed with that reason, a
   *  saved choice the verdict refuses stays selected and says why under the field, and
   *  nothing can be picked while the verdict is loading or failed. */
  availability?: ModelAvailability
}) {
  const { t } = useTranslation('models')
  const id = useId()
  const { models, selected, unavailable, isPending, isError } = useStageSelection(stage)
  const save = useSaveSelection()
  const selectedVerdict = selected ? verdictOf(availability, selected) : undefined

  // The saved choice's key when it can be shown as chosen; the greyed unusable entry
  // otherwise. An empty value is the placeholder.
  const value = selected ? refKey(selected) : unavailable ? UNAVAILABLE_VALUE : ''
  const labelId = `${id}-label`
  const loadErrorId = `${id}-load-error`
  const saveErrorId = `${id}-save-error`
  const unavailableId = `${id}-unavailable`
  const availabilityId = `${id}-availability`
  const availabilityNote =
    availability?.kind === 'loading'
      ? t('availability.loading')
      : availability?.kind === 'failed'
        ? t('availability.failed')
        : selectedVerdict && !selectedVerdict.usable
          ? selectedVerdict.reason || t('availability.unresolved')
          : ''
  const describedBy = [
    isError && loadErrorId,
    save.failure && saveErrorId,
    unavailable && unavailableId,
    availabilityNote && availabilityId,
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
    ...models.map((model) => {
      const verdict = verdictOf(availability, model.ref)
      return {
        value: refKey(model.ref),
        label: optionLabel(model, stage, verdict.usable ? '' : verdict.reason),
        disabled:
          model.disabled ||
          (!availability && !model.affordable) ||
          (requireVideoInput && !model.videoInput) ||
          !verdict.usable,
      }
    }),
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
        disabled={disabled || isPending || save.isPending}
        aria-invalid={isError || Boolean(save.failure) || undefined}
        aria-describedby={describedBy || undefined}
        onChange={(next) => {
          const chosen = models.find((model) => refKey(model.ref) === next)
          if (
            !disabled &&
            chosen &&
            !chosen.disabled &&
            (availability || chosen.affordable) &&
            (!requireVideoInput || chosen.videoInput) &&
            verdictOf(availability, chosen.ref).usable
          )
            save.save(stage, chosen.ref)
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
      {availabilityNote && (
        // The workflow's verdict on the saved choice, or the state of the verdict itself. The
        // choice stays selected — it may still serve other work — so this is a standing note,
        // not an alert, and it may run to several lines at 360 px rather than be cut.
        <FieldMessage id={availabilityId} role="status" className="mt-1 break-words">
          {availabilityNote}
          {availability?.kind === 'failed' && availability.retry && (
            <>
              {' '}
              <Button type="button" variant="ghost" size="compact" onClick={availability.retry}>
                {t('availability.retry')}
              </Button>
            </>
          )}
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
function optionLabel(model: CatalogModel, stage: StageName, refusal = ''): string {
  const badges = [
    model.vision && '👁',
    // Watching a clip is a capability of its own, and a post with a video needs it of the
    // observe model (VIDEO-11) — so the picker says which models have it.
    model.videoInput && i18next.t('capability.video', { ns: 'models' }),
    model.structuredOutput && i18next.t('selectField.structuredOutput', { ns: 'models' }),
  ]
    .filter(Boolean)
    .join(' ')
  // A locked model stays listed rather than vanishing, and says which tier unlocks it: the
  // reason it cannot be chosen is the only thing this entry has to teach. A provider without
  // a key is the more immediate obstacle, so that reason wins when both apply.
  // The workflow's own refusal (T112) comes after the provider's state: a model with no key is
  // unusable everywhere, which is the more immediate thing to say.
  const reason = model.disabled
    ? ` (${model.disabledReason})`
    : refusal
      ? ` (${refusal})`
      : !model.affordable
        ? ` (${i18next.t('selectField.unaffordable', { ns: 'models', credits: model.requiredCredits })})`
        : ''
  // The grade LEADS: the closed trigger truncates, so a trailing one is never read.
  return `${levelPrefix(model, stage)}${model.label}${badges ? ` ${badges}` : ''}${reason}`
}
