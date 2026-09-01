import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  filterForStage,
  refKey,
  type ModelRef,
  type StageName,
  useModels,
  useModelSetup,
  useSaveComparisonPair,
  useSaveSelection,
  useStageSelection,
} from '@/entities/model-catalog'
import { formatNumber } from '@/shared/lib'
import type { AppFailure } from '@/shared/api'
import {
  AppFailureMessage,
  Button,
  FieldLabel,
  FieldMessage,
  Select,
  Typography,
} from '@/shared/ui'

export function ModelPairForm({ stage }: { stage: StageName }) {
  const { models } = useModels()
  const active = useStageSelection(stage)
  const { pairs } = useModelSetup()
  const saveActive = useSaveSelection()
  const savePair = useSaveComparisonPair()
  const suitable = filterForStage(models, stage)
  const pair = pairs.find((item) => item.stage === stage)
  const initialA = pair?.candidateA ? refKey(pair.candidateA.ref) : ''
  const initialB = pair?.candidateB ? refKey(pair.candidateB.ref) : ''

  return (
    <ModelPairFields
      key={`${stage}:${initialA}:${initialB}`}
      stage={stage}
      initialA={initialA}
      initialB={initialB}
      suitable={suitable}
      active={active}
      saveActive={saveActive}
      savePair={savePair}
    />
  )
}

function ModelPairFields({
  stage,
  initialA,
  initialB,
  suitable,
  active,
  saveActive,
  savePair,
}: {
  stage: StageName
  initialA: string
  initialB: string
  suitable: ReturnType<typeof useModels>['models']
  active: ReturnType<typeof useStageSelection>
  saveActive: ReturnType<typeof useSaveSelection>
  savePair: ReturnType<typeof useSaveComparisonPair>
}) {
  const { t } = useTranslation('models')
  const [a, setA] = useState(initialA)
  const [b, setB] = useState(initialB)
  const find = (key: string): ModelRef | undefined =>
    suitable.find((model) => refKey(model.ref) === key)?.ref
  const invalid = !a || !b || a === b || !find(a) || !find(b)
  return (
    <div className="space-y-4">
      <ModelSelect
        label={t('active')}
        value={active.selected ? refKey(active.selected) : ''}
        models={suitable}
        onChange={(key) => {
          const ref = find(key)
          if (ref) void saveActive.save(stage, ref)
        }}
        // This field is controlled by the SERVER value, so a save that fails snaps the choice back
        // to the previous model on its own. On touch the native picker has just closed and this
        // field is the only thing on screen the user is looking at, so both the in-flight state and
        // the failure have to render right here or the undo has no explanation at all (§6).
        saving={saveActive.isPending}
        error={saveActive.failure}
      />
      <div className="grid gap-4 sm:grid-cols-2">
        <ModelSelect label={t('candidateA')} value={a} models={suitable} onChange={setA} />
        <ModelSelect label={t('candidateB')} value={b} models={suitable} onChange={setB} />
      </div>
      {a && a === b && <FieldMessage>{t('differentModels')}</FieldMessage>}
      <div>
        <Button
          variant="secondary"
          className="w-full sm:w-auto"
          disabled={invalid}
          pending={savePair.isPending}
          onClick={() => {
            const left = find(a)
            const right = find(b)
            if (left && right) {
              void savePair.save(stage, left, right).catch(() => {
                // The mutation state carries the structured failure rendered below.
              })
            }
          }}
        >
          {t('savePair')}
        </Button>
        {/* This is a commit point ~940px down the page, and the '비교 시작' CTA that depends on it
            is another ~160px below. Without these two lines a failed save just returns the button
            to rest — indistinguishable from success — and the user only learns something is wrong
            from a CTA that stays disabled two screens away (§4.3). */}
        {savePair.failure && (
          <Typography
            variant="body"
            as="div"
            role="alert"
            className="text-field-error mt-2 break-words"
          >
            <AppFailureMessage failure={savePair.failure} />
          </Typography>
        )}
        {savePair.isSuccess && (
          <Typography variant="body" role="status" className="text-content-secondary mt-2">
            {t('pair.saved')}
          </Typography>
        )}
      </div>
    </div>
  )
}

function ModelSelect({
  label,
  value,
  models,
  onChange,
  saving = false,
  error,
}: {
  label: string
  value: string
  models: ReturnType<typeof useModels>['models']
  onChange: (value: string) => void
  /** A save of this field is in flight. Rendered in place under the field, not just implied by
   *  the control greying out. */
  saving?: boolean
  /** Why the last save of this field failed, if it did. */
  error?: AppFailure
}) {
  const { t } = useTranslation('models')
  const id = useId()
  const errorId = `${id}-error`
  return (
    <div>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <Select
        id={id}
        className="mt-1"
        value={value}
        // Disabled only while a save is in flight: on 3G the round trip is seconds long and a
        // second tap would fire a second SaveSelection against the first one's result.
        disabled={saving}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? errorId : undefined}
        onChange={(event) => onChange(event.target.value)}
      >
        <option value="">{t('select')}</option>
        {models.map((model) => (
          <option key={refKey(model.ref)} value={refKey(model.ref)} disabled={model.disabled}>
            {model.label}
            {model.disabled ? ` · ${model.disabledReason}` : ''}
          </option>
        ))}
      </Select>
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
      {error && (
        <Typography
          variant="body"
          as="div"
          id={errorId}
          role="alert"
          className="text-field-error mt-1 break-words"
        >
          <AppFailureMessage failure={error} />
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
