import i18next from 'i18next'
import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  filterForStage,
  refKey,
  type CatalogModel,
  type ModelRef,
  type StageName,
  useModels,
  useModelSetup,
  useSaveComparisonPair,
} from '@/entities/model-catalog'
import { planLabel } from '@/entities/plan'
import {
  AppFailureMessage,
  FieldLabel,
  FieldMessage,
  Listbox,
  Typography,
  type ListboxOption,
} from '@/shared/ui'

/** The A/B pair for one stage, as TWO dropdowns side by side, saved the moment both name a
 *  different model.
 *
 *  `ModelPairForm` is the same pair on the AI 모델 page, where it sits under the active model with
 *  its own 저장 button because that page is a settings form the user commits. This one is for the
 *  writing brief, which the editor opens over a draft: it used to be a LINK to that page, and
 *  following it mid-draft cost the user their place for a choice that is two dropdowns wide
 *  (owner decision 2026-09-02). A surface with no commit control cannot ask for one, so the save
 *  rides the second choice — the same way every other field of the brief saves itself.
 *
 *  Two equal models are not saved: the backend refuses that pair, and the message under the
 *  fields is the whole correction. */
export function CandidatePairSelect({
  stage,
  className,
}: {
  stage: StageName
  className?: string
}) {
  const { models, isPending, isError } = useModels()
  const { pairs } = useModelSetup()
  const savePair = useSaveComparisonPair()
  const pair = pairs.find((item) => item.stage === stage)
  const savedA = pair?.candidateA ? refKey(pair.candidateA.ref) : ''
  const savedB = pair?.candidateB ? refKey(pair.candidateB.ref) : ''
  return (
    <CandidatePairFields
      // The server's pair is the truth about what the next A/B run uses, so a pair changed on the
      // AI 모델 page (or by applying a recommendation) reseeds these fields rather than leaving
      // this surface showing a choice nobody made here.
      key={`${stage}:${savedA}:${savedB}`}
      stage={stage}
      savedA={savedA}
      savedB={savedB}
      suitable={filterForStage(models, stage)}
      disabled={isPending || isError}
      savePair={savePair}
      className={className}
    />
  )
}

function CandidatePairFields({
  stage,
  savedA,
  savedB,
  suitable,
  disabled,
  savePair,
  className,
}: {
  stage: StageName
  savedA: string
  savedB: string
  suitable: readonly CatalogModel[]
  disabled: boolean
  savePair: ReturnType<typeof useSaveComparisonPair>
  className?: string
}) {
  const { t } = useTranslation('models')
  const [a, setA] = useState(savedA)
  const [b, setB] = useState(savedB)
  const find = (key: string): ModelRef | undefined =>
    suitable.find((model) => refKey(model.ref) === key)?.ref

  const commit = (nextA: string, nextB: string) => {
    setA(nextA)
    setB(nextB)
    if (!nextA || !nextB || nextA === nextB) return
    const left = find(nextA)
    const right = find(nextB)
    if (!left || !right) return
    void savePair.save(stage, left, right).catch(() => {
      // The mutation's own failure is rendered under the fields.
    })
  }

  // NO blank entry, unlike the same fields on the AI 모델 page. A pair is two distinct registered
  // models or it has never been set — `SaveComparisonPair` refuses an empty ref and there is no
  // RPC that clears one — so choosing 모델을 선택하세요 here could only empty the FIELD. The saved
  // pair would stay exactly as it was, and 글 생성's A/B 비교 would go on running the candidate the
  // user had just watched disappear. The page's copy of this form can afford the entry because its
  // 저장 button visibly disables and says the choice is not in effect; a surface whose fields save
  // themselves has nowhere to say it, so it does not offer the choice at all. Not-yet-set is the
  // placeholder on an empty value, which matches no option.
  const options: ListboxOption<string>[] = suitable.map((model) => ({
    value: refKey(model.ref),
    label: optionLabel(model),
    disabled: model.disabled || model.locked,
  }))

  return (
    <div className={className}>
      {/* Left and right, never stacked: A and B are one choice made twice, and a two-row form
          reads as two unrelated fields. They stay side by side on the narrowest phone — the
          triggers truncate, and the open panel is where a model's full name is read anyway. */}
      <div className="grid grid-cols-2 gap-3">
        <CandidateField
          label={t('candidateA')}
          value={a}
          options={options}
          placeholder={t('select')}
          disabled={disabled || savePair.isPending}
          onChange={(next) => commit(next, b)}
        />
        <CandidateField
          label={t('candidateB')}
          value={b}
          options={options}
          placeholder={t('select')}
          disabled={disabled || savePair.isPending}
          onChange={(next) => commit(a, next)}
        />
      </div>
      {a && a === b && <FieldMessage className="mt-1">{t('differentModels')}</FieldMessage>}
      {/* Mounted while idle so it announces when it fills, and out of the layout until it does. */}
      <Typography
        variant="body"
        as="p"
        role="status"
        className="text-content-tertiary mt-1 empty:hidden"
      >
        {savePair.isPending ? t('pair.saving') : null}
      </Typography>
      {savePair.failure && (
        <Typography
          variant="body"
          as="div"
          role="alert"
          className="text-field-error mt-1 break-words"
        >
          <AppFailureMessage failure={savePair.failure} />
        </Typography>
      )}
    </div>
  )
}

function CandidateField({
  label,
  value,
  options,
  placeholder,
  disabled,
  onChange,
}: {
  label: string
  value: string
  options: ListboxOption<string>[]
  /** Shown for a candidate that has never been set. It is the field's EMPTY STATE, not a listed
   *  choice — see the option list above for why the difference matters here. */
  placeholder: string
  disabled: boolean
  onChange: (value: string) => void
}) {
  const id = useId()
  const labelId = `${id}-label`
  return (
    <div className="min-w-0">
      <FieldLabel id={labelId} htmlFor={id}>
        {label}
      </FieldLabel>
      <Listbox
        id={id}
        aria-labelledby={labelId}
        className="mt-1"
        value={value}
        options={options}
        placeholder={placeholder}
        disabled={disabled}
        onChange={onChange}
      />
    </div>
  )
}

/** The reason a model cannot be chosen, appended to its name. Only ever read inside the OPEN
 *  panel: `onChange` refuses a disabled row, so it can never become the trigger's value. A
 *  provider without a key is the more immediate obstacle, so that reason wins over the plan. */
function optionLabel(model: CatalogModel): string {
  if (model.disabled) return `${model.label} (${model.disabledReason})`
  if (model.locked) {
    const reason = i18next.t('selectField.locked', {
      ns: 'models',
      plan: planLabel(model.minPlan),
    })
    return `${model.label} (${reason})`
  }
  return model.label
}
