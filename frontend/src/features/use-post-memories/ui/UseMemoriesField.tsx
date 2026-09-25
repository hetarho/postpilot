import { useTranslation } from 'react-i18next'
import { Checkbox, typographyStyles } from '@/shared/ui'

/** The memory opt-in (POST-71, MEM-18, MEM-26), a control of the writing brief's run-options form:
 *  it reports the toggle to the form and saves nothing — the brief's 저장 saves it with the rest of
 *  the set (POST-89). */
export function UseMemoriesField({
  checked,
  onChange,
  disabled,
  className,
}: {
  checked: boolean
  onChange: (next: boolean) => void
  disabled?: boolean
  className?: string
}) {
  const { t } = useTranslation('posts')
  return (
    <div className={className}>
      {/* The primitive owns the 44px target; the label wraps it so the words are part of it. */}
      <label
        className={typographyStyles({
          variant: 'label',
          className: 'flex min-h-11 items-center gap-3',
        })}
      >
        <Checkbox
          checked={checked}
          disabled={disabled}
          onChange={(event) => onChange(event.target.checked)}
        />
        {t('useMemories.label')}
      </label>
    </div>
  )
}
