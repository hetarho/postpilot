import { useTranslation } from 'react-i18next'
import { FieldLabel, Listbox } from '@/shared/ui'
import { MEMORY_KINDS, type MemoryKind } from '../model/types'

/** The kind control, here rather than in a feature because the create sheet and the row edit
 *  both need the identical one and a feature may not import a sibling feature (ARCH-13).
 *
 *  Five options and no "unset": a kind decides whether the fact is a candidate for every post or
 *  only on tag overlap (MEM-6), so it is chosen, never defaulted by the server. */
export function MemoryKindField({
  id,
  value,
  onChange,
  disabled,
  className,
}: {
  id: string
  value: MemoryKind
  onChange: (next: MemoryKind) => void
  disabled?: boolean
  className?: string
}) {
  const { t } = useTranslation('memories')
  const labelId = `${id}-label`
  return (
    <div className={className}>
      <FieldLabel id={labelId} htmlFor={id}>
        {t('field.kind')}
      </FieldLabel>
      <Listbox
        id={id}
        value={value}
        aria-labelledby={labelId}
        options={MEMORY_KINDS.map((kind) => ({ value: kind, label: t(`kind.${kind}`) }))}
        onChange={onChange}
        disabled={disabled}
        className="mt-1"
      />
    </div>
  )
}
