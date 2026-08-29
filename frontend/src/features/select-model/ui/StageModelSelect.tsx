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
  const id = useId()
  const { models, selected, unavailable, isPending, isError } = useStageSelection(stage)
  const save = useSaveSelection()

  // The saved choice's key when it can be shown as chosen; the greyed unusable entry
  // otherwise. An empty value is the placeholder.
  const value = selected ? refKey(selected) : unavailable ? UNAVAILABLE_VALUE : ''
  const loadErrorId = `${id}-load-error`
  const saveErrorId = `${id}-save-error`
  const unavailableId = `${id}-unavailable`
  const describedBy = [
    isError && loadErrorId,
    save.isError && saveErrorId,
    unavailable && unavailableId,
  ]
    .filter(Boolean)
    .join(' ')

  return (
    <div className={className}>
      <FieldLabel htmlFor={id}>
        {STAGE_LABELS[stage]} 모델{optional ? ' (선택)' : ''}
      </FieldLabel>
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
          // An option's text is the CHOICE, not the explanation (§7). This entry is the select's
          // current value, so its whole string has to fit the CLOSED control — ~284px at 360px,
          // which a `provider/model` path alone already fills. The native control would ellipsise
          // the reason away, which is the only thing this entry exists to say, so the reason goes
          // in the message slot under the field instead.
          <option value={UNAVAILABLE_VALUE} disabled>
            {refKey(unavailable.ref)}
          </option>
        )}
        {models.map((model) => (
          <option key={refKey(model.ref)} value={refKey(model.ref)} disabled={model.disabled}>
            {optionLabel(model)}
          </option>
        ))}
      </Select>
      {/* Visible, not sr-only: the control greys out for the 1–3s a SaveSelection takes on mobile
          data, and a touch user watching the field it just closed over is exactly who needs the
          cause (§6). The region stays mounted so it announces when it fills, and `empty:hidden`
          keeps it out of the layout while it is idle. */}
      <p role="status" className="text-content-tertiary mt-1 text-xs empty:hidden">
        {save.isPending ? '선택을 저장하는 중…' : null}
      </p>
      {unavailable && (
        // `status`, not the default `alert`: this is a standing condition of the saved value, not
        // something that just went wrong, and it renders on first paint.
        <FieldMessage id={unavailableId} role="status" className="mt-1 text-xs">
          {unavailable.reason}
        </FieldMessage>
      )}
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

/** `<label> 👁 구조화 응답` badges for what the model can do (PRD §6.4), and the reason when it
 *  cannot be chosen. Plain text: an <option> renders no markup.
 *
 *  These strings are only ever read inside the OPEN picker, which is a full-screen sheet on both
 *  phone platforms — a disabled option can never become the closed control's value, because
 *  `onChange` refuses it and an unusable saved choice is rendered as the separate entry above. */
function optionLabel(model: CatalogModel): string {
  const badges = [model.vision && '👁', model.structuredOutput && '구조화 응답']
    .filter(Boolean)
    .join(' ')
  const reason = model.disabled ? ` — ${model.disabledReason}` : ''
  return `${model.label}${badges ? ` ${badges}` : ''}${reason}`
}
