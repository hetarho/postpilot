import { useId, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import {
  MAX_RECOMMENDATION_LABEL_LENGTH,
  RECOMMENDATION_STAGES,
  filterForStage,
  isRecommendationFieldCause,
  levelPrefix,
  recommendationField,
  recommendationSlots,
  refKey,
  useModels,
  useSaveRecommendationSet,
  type ModelRef,
  type RecommendationSet,
  type SelectionSlotName,
  type StageName,
} from '@/entities/model-catalog'
import {
  AppFailureMessage,
  Button,
  FieldLabel,
  FieldMessage,
  Listbox,
  Notice,
  TextField,
  Typography,
  type ListboxOption,
} from '@/shared/ui'

type Slots = Partial<Record<string, ModelRef>>

function slotKey(stage: StageName, slot: SelectionSlotName): string {
  return `${stage}.${slot}`
}

function slotsOf(set: RecommendationSet): Slots {
  const slots: Slots = {}
  for (const selection of set.selections) {
    for (const slot of recommendationSlots(selection.stage)) {
      const ref = selection[slot]
      if (ref) slots[slotKey(selection.stage, slot)] = ref
    }
  }
  return slots
}

/** One set as a draft: a name and the seven slots (MODEL-69).
 *
 *  The whole set is sent on 저장 and the server validates it whole (MODEL-70), so the form keeps
 *  no rule of its own beyond the label's length: a refusal names every offending field at once and
 *  each cause is shown beside its own field, with the draft kept as it was. The options are the
 *  models registered AND classified for the stage — exactly what the server will accept — while a
 *  saved value that no longer qualifies stays visible as a disabled option, so the operator sees
 *  what they are replacing. */
export function RecommendationSetEditor({
  initial,
  onDone,
}: {
  initial: RecommendationSet
  onDone: () => void
}) {
  const { t } = useTranslation('models')
  const id = useId()
  const { models } = useModels()
  const save = useSaveRecommendationSet()
  const [label, setLabel] = useState(initial.label)
  const [slots, setSlots] = useState<Slots>(() => slotsOf(initial))

  const causes: Readonly<Record<string, string>> =
    save.failure?.reason === 'MODEL_SET_INVALID' ? save.failure.params : {}
  const causeText = (field: string) => {
    const cause = causes[field]
    return cause && isRecommendationFieldCause(cause)
      ? t(`recommendationSets.cause.${cause}`, { max: MAX_RECOMMENDATION_LABEL_LENGTH })
      : ''
  }

  const submit = (event: FormEvent) => {
    event.preventDefault()
    const draft: RecommendationSet = {
      id: initial.id,
      label,
      selections: RECOMMENDATION_STAGES.map((stage) => ({
        stage,
        active: slots[slotKey(stage, 'active')] ?? { providerId: '', modelId: '' },
        candidateA: slots[slotKey(stage, 'candidateA')],
        candidateB: slots[slotKey(stage, 'candidateB')],
      })),
    }
    void save
      .save(draft)
      .then(onDone)
      .catch(() => {
        // The mutation state carries the structured failure rendered in this form.
      })
  }

  const headingId = `${id}-heading`
  const labelError = causeText('label')
  return (
    <form
      aria-labelledby={headingId}
      onSubmit={submit}
      className="bg-surface-raised grid gap-4 rounded-lg p-4"
    >
      <Typography variant="fieldTitle" as="h3" id={headingId}>
        {t(
          initial.id ? 'recommendationSets.editor.editTitle' : 'recommendationSets.editor.newTitle',
        )}
      </Typography>

      {save.failure && (
        <Notice tone="danger" role="alert">
          <AppFailureMessage failure={save.failure} />
        </Notice>
      )}

      <div>
        <FieldLabel htmlFor={`${id}-label`}>{t('recommendationSets.editor.label')}</FieldLabel>
        <TextField
          id={`${id}-label`}
          className="mt-1"
          value={label}
          maxLength={MAX_RECOMMENDATION_LABEL_LENGTH}
          aria-invalid={labelError !== '' || undefined}
          aria-describedby={labelError ? `${id}-label-error` : undefined}
          onChange={(event) => setLabel(event.target.value)}
        />
        {labelError && (
          <FieldMessage id={`${id}-label-error`} role="status" className="mt-1">
            {labelError}
          </FieldMessage>
        )}
      </div>

      {RECOMMENDATION_STAGES.map((stage) => (
        <fieldset key={stage} className="grid gap-3">
          <Typography variant="label" as="legend" className="mb-1">
            {t(`recommendationSets.stageGroup.${stage}`)}
          </Typography>
          <div className="grid gap-3 sm:grid-cols-3">
            {recommendationSlots(stage).map((slot) => {
              const fieldId = `${id}-${stage}-${slot}`
              const error = causeText(recommendationField(stage, slot))
              return (
                <SlotPicker
                  key={slot}
                  id={fieldId}
                  stage={stage}
                  label={t('recommendationSets.slotLabel', {
                    stage: t(`recommendationSets.stageGroup.${stage}`),
                    slot: t(`recommendationSets.slot.${slot}`),
                  })}
                  shortLabel={t(`recommendationSets.slot.${slot}`)}
                  value={slots[slotKey(stage, slot)]}
                  models={models}
                  error={error}
                  disabled={save.isPending}
                  onChange={(ref) =>
                    setSlots((current) => ({ ...current, [slotKey(stage, slot)]: ref }))
                  }
                />
              )
            })}
          </div>
        </fieldset>
      ))}

      <div className="flex flex-wrap gap-2">
        <Button type="submit" variant="cta" pending={save.isPending}>
          {t('recommendationSets.editor.save')}
        </Button>
        <Button type="button" variant="ghost" disabled={save.isPending} onClick={onDone}>
          {t('recommendationSets.editor.cancel')}
        </Button>
      </div>
    </form>
  )
}

function SlotPicker({
  id,
  stage,
  label,
  shortLabel,
  value,
  models,
  error,
  disabled,
  onChange,
}: {
  id: string
  stage: StageName
  label: string
  shortLabel: string
  value: ModelRef | undefined
  models: Parameters<typeof filterForStage>[0]
  error: string
  disabled: boolean
  onChange: (ref: ModelRef) => void
}) {
  const { t } = useTranslation('models')
  const candidates = filterForStage(models, stage)
  const options: ListboxOption<string>[] = candidates.map((model) => ({
    value: refKey(model.ref),
    label: `${levelPrefix(model, stage)}${model.label}`,
  }))
  const current = value ? refKey(value) : ''
  if (value && !candidates.some((model) => refKey(model.ref) === current)) {
    options.push({
      value: current,
      label: t('recommendationSets.editor.retired', { model: value.modelId }),
      disabled: true,
    })
  }
  const labelId = `${id}-label`
  const errorId = `${id}-error`
  return (
    <div className="min-w-0">
      <FieldLabel id={labelId} htmlFor={id}>
        <span aria-hidden="true">{shortLabel}</span>
        <span className="sr-only">{label}</span>
      </FieldLabel>
      <Listbox
        id={id}
        aria-labelledby={labelId}
        aria-invalid={error !== '' || undefined}
        aria-describedby={error ? errorId : undefined}
        className="mt-1"
        value={current}
        placeholder={t('recommendationSets.editor.choose')}
        options={options}
        disabled={disabled}
        onChange={(chosen) => {
          const model = candidates.find((candidate) => refKey(candidate.ref) === chosen)
          if (model) onChange(model.ref)
        }}
      />
      {error && (
        <FieldMessage id={errorId} role="status" className="mt-1">
          {error}
        </FieldMessage>
      )}
    </div>
  )
}
