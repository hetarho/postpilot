import { useId } from 'react'
import { clsx } from 'clsx'
import {
  type CatalogModel,
  STAGE_LABELS,
  type StageName,
  refKey,
  useSaveSelection,
  useStageSelection,
} from '@/entities/model-catalog'

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

  return (
    <label htmlFor={id} className={clsx('flex flex-col gap-1 text-xs text-text-muted', className)}>
      <span>{STAGE_LABELS[stage]} 모델</span>
      <select
        id={id}
        value={value}
        disabled={isPending || save.isPending}
        aria-invalid={isError || save.isError || undefined}
        onChange={(event) => {
          const chosen = models.find((model) => refKey(model.ref) === event.target.value)
          if (chosen && !chosen.disabled) save.save(stage, chosen.ref)
        }}
        className="rounded-md bg-surface-raised px-2 py-1.5 text-sm text-text disabled:opacity-50"
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
      </select>
      {isError && (
        <span role="alert" className="text-danger">
          모델 목록을 불러오지 못했어요.
        </span>
      )}
      {save.isError && (
        <span role="alert" className="text-danger">
          선택을 저장하지 못했어요. 다시 골라 주세요.
        </span>
      )}
    </label>
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
