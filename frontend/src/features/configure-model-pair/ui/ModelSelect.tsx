import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import {
  refKey,
  levelPrefix,
  modelChoiceIssue,
  type StageName,
  type useModels,
} from '@/entities/model-catalog'
import { formatNumber } from '@/shared/lib'
import type { AppFailure } from '@/shared/api'
import { AppFailureMessage, FieldLabel, Listbox, Typography } from '@/shared/ui'

export function ModelSelect({
  label,
  stage,
  value,
  models,
  onChange,
  saving = false,
  error,
  notice,
  savedIssue,
}: {
  label: string
  /** Which stage's grade to show: a model is graded per stage, not once (MODEL-57). */
  stage: StageName
  value: string
  models: ReturnType<typeof useModels>['models']
  onChange: (value: string) => void
  /** A save of this field is in flight. Rendered in place under the field, not just implied by
   *  the control greying out. */
  saving?: boolean
  /** Why the last save of this field failed, if it did. */
  error?: AppFailure
  /** Why the last change of this field wrote nothing (MODEL-65). */
  notice?: string
  savedIssue?: string
}) {
  const { t } = useTranslation('models')
  const id = useId()
  const labelId = `${id}-label`
  const errorId = `${id}-error`
  return (
    <div>
      <FieldLabel id={labelId} htmlFor={id}>
        {label}
      </FieldLabel>
      <Listbox
        id={id}
        aria-labelledby={labelId}
        className="mt-1"
        value={value}
        options={[
          { value: '', label: t('select') },
          ...(value && !models.some((model) => refKey(model.ref) === value)
            ? [{ value, label: `${value} · ${savedIssue || t('unsuitable')}`, disabled: true }]
            : []),
          ...models.map((model) => ({
            value: refKey(model.ref),
            // The grade leads here too, so the three fields that render this same list
            // read identically (MODEL-44).
            label: `${levelPrefix(model, stage)}${model.label}${modelChoiceIssue(model, stage) ? ` · ${modelChoiceIssue(model, stage)}` : ''}`,
            disabled: Boolean(modelChoiceIssue(model, stage)),
          })),
        ]}
        // Disabled only while a save is in flight: on 3G the round trip is seconds long and a
        // second tap would fire a second SaveSelection against the first one's result.
        disabled={saving}
        aria-invalid={error || notice ? true : undefined}
        aria-describedby={error || notice ? errorId : undefined}
        onChange={onChange}
      />
      {value && <ModelMeta model={models.find((model) => refKey(model.ref) === value)} />}
      {/* The live region stays mounted so it announces when it fills, and `empty:hidden` keeps it
          out of the layout while it is idle. */}
      <Typography
        variant="body"
        as="p"
        role="status"
        className="text-content-tertiary mt-1 empty:hidden"
      >
        {saving ? t('pair.saving') : null}
      </Typography>
      {(error || notice) && (
        <Typography
          variant="body"
          as="div"
          id={errorId}
          role="alert"
          className="text-field-error mt-1 break-words"
        >
          {error ? <AppFailureMessage failure={error} /> : notice}
        </Typography>
      )}
    </div>
  )
}

function ModelMeta({
  model,
}: {
  model: ReturnType<typeof useModels>['models'][number] | undefined
}) {
  const { t } = useTranslation('models')
  if (!model) return null
  const context = formatNumber(Number(model.contextTokens))
  return (
    <Typography variant="label" as="p" className="mt-1">
      {t('pair.pricing', {
        tokens: context,
        input: model.inputUsdPerMillion || '?',
        output: model.outputUsdPerMillion || '?',
      })}
      {/* The date the price was checked is provenance, not a decision input, and it is what pushed
          this line to two rows under each of the three selects — 72px of the fold, three times
          over, on a 360px phone. It appears only where there is width for it. */}
      <span className="hidden sm:inline">
        {' · '}
        {model.pricingCheckedAt || t('pair.priceUnchecked')}
      </span>
    </Typography>
  )
}
