import { useId } from 'react'
import {
  type CatalogModel,
  STAGE_LABELS,
  type StageName,
  refKey,
  useSaveSelection,
  useStageSelection,
} from '@/entities/model-catalog'
import { FieldLabel, FieldMessage, Select } from '@/shared/ui'

/** The per-stage model dropdown (PRD §3.3, §6.4, F-4).
 *
 *  Lists what the registry offers for the stage — vision models only for `observe` —
 *  with a disabled model greyed and its reason shown, a vanished saved choice greyed
 *  with "사라졌어요", and no pre-selection: until the user picks, the stage is empty and
 *  `useStageSelection(stage).selected` is null, which is what the generation and analysis
 *  actions block on ([I3]). Pages mount it; features never import it. */
export function StageModelSelect({ stage, className }: { stage: StageName; className?: string }) {
  const id = useId()
  const { models, selected, unavailable, isPending, isError } = useStageSelection(stage)
  const save = useSaveSelection()

  // The saved choice's key when it can be shown as chosen; the greyed unusable entry
  // otherwise. An empty value is the placeholder.
  const value = selected ? refKey(selected) : unavailable ? UNAVAILABLE_VALUE : ''
  const loadErrorId = `${id}-load-error`
  const saveErrorId = `${id}-save-error`
  const describedBy = [isError && loadErrorId, save.isError && saveErrorId]
    .filter(Boolean)
    .join(' ')

  return (
    <div className={className}>
      <FieldLabel htmlFor={id}>{STAGE_LABELS[stage]} 모델</FieldLabel>
      <Select
        id={id}
        value={value}
        disabled={isPending || save.isPending}
        aria-invalid={isError || save.isError || undefined}
        aria-describedby={describedBy || undefined}
        onChange={(event) => {
          const chosen = models.find((model) => refKey(model.ref) === event.target.value)
          if (chosen && !chosen.disabled) save.save(stage, chosen.ref)
        }}
        className="mt-1"
      >
        <option value="">모델을 선택하세요</option>
        {unavailable && (
          <option value={UNAVAILABLE_VALUE} disabled>
            {refKey(unavailable.ref)} — {unavailable.reason}
          </option>
        )}
        {models.map((model) => (
          <option key={refKey(model.ref)} value={refKey(model.ref)} disabled={model.disabled}>
            {optionLabel(model)}
          </option>
        ))}
      </Select>
      <span role="status" className="sr-only">
        {save.isPending ? '선택을 저장하는 중…' : ''}
      </span>
      {isError && (
        <FieldMessage id={loadErrorId} className="mt-1 text-xs">
          모델 목록을 불러오지 못했어요.
        </FieldMessage>
      )}
      {save.isError && (
        <FieldMessage id={saveErrorId} className="mt-1 text-xs">
          선택을 저장하지 못했어요. 다시 골라 주세요.
        </FieldMessage>
      )}
    </div>
  )
}

const UNAVAILABLE_VALUE = '__unavailable__'

/** `<label> 👁 {}` badges for what the model can do (PRD §6.4), and the reason when it
 *  cannot be chosen. Plain text: an <option> renders no markup. */
function optionLabel(model: CatalogModel): string {
  const badges = [model.vision && '👁', model.structuredOutput && '{}'].filter(Boolean).join(' ')
  const reason = model.disabled ? ` — ${model.disabledReason}` : ''
  return `${model.label}${badges ? ` ${badges}` : ''}${reason}`
}
