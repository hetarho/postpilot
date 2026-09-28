import { useTranslation } from 'react-i18next'
import { FieldLabel, FieldMessage, TextField } from '@/shared/ui'
import { remainingGuidelineTitleChars } from '../model/types'

/** A guideline's optional title (GUIDE-46): the list's name for the rule, never part of a prompt.
 *  Here rather than in a feature because every surface that creates a guideline offers it — the
 *  create sheet, 승인, 지침으로 저장 and 영상 지침으로 저장 — and a feature may not import a
 *  sibling. It is optional, so it carries no standing counter and no help line; only a title past
 *  the bound says so, and the caller disables its save on `remainingGuidelineTitleChars < 0`. */
export function GuidelineTitleField({
  id,
  value,
  onChange,
  disabled,
  autoFocus,
  className,
}: {
  id: string
  value: string
  onChange: (next: string) => void
  disabled?: boolean
  autoFocus?: boolean
  className?: string
}) {
  const { t } = useTranslation(['guidelines', 'common'])
  const left = remainingGuidelineTitleChars(value)
  const exceeded = left < 0
  const countId = `${id}-count`
  return (
    <div className={className}>
      <FieldLabel htmlFor={id}>{t('titleField.label', { ns: 'guidelines' })}</FieldLabel>
      <TextField
        id={id}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        disabled={disabled}
        // Inside a Sheet the panel takes focus on open unless a control is marked for it.
        autoFocus={autoFocus}
        data-autofocus={autoFocus || undefined}
        type="text"
        inputMode="text"
        autoComplete="off"
        autoCapitalize="off"
        autoCorrect="off"
        enterKeyHint="next"
        aria-invalid={exceeded || undefined}
        aria-describedby={exceeded ? countId : undefined}
        className="mt-1"
      />
      {exceeded && (
        <FieldMessage id={countId} role="status" className="mt-2">
          {t('count.exceeded', { ns: 'common', count: -left })}
        </FieldMessage>
      )}
    </div>
  )
}
