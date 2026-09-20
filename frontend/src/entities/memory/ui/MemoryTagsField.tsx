import { useTranslation } from 'react-i18next'
import { FieldLabel, FieldMessage, TextField, Typography } from '@/shared/ui'
import { MEMORY_LIMITS, parseMemoryTags } from '../model/types'

/** The tag control, beside the kind control and here for the same reason: both surfaces that
 *  author a memory need the identical one.
 *
 *  Comma-separated, like the post editor's tag field. Tags are what retrieval matches against
 *  (MEM-7), so the help text says what they are for rather than what they look like — and the
 *  count is stated because going over is a refusal, not a truncation. */
export function MemoryTagsField({
  id,
  value,
  onChange,
  disabled,
  className,
}: {
  id: string
  value: string
  onChange: (next: string) => void
  disabled?: boolean
  className?: string
}) {
  const { t } = useTranslation('memories')
  const helpId = `${id}-help`
  const tags = parseMemoryTags(value)
  const tooMany = tags.length > MEMORY_LIMITS.tags
  return (
    <div className={className}>
      <FieldLabel htmlFor={id}>{t('field.tags')}</FieldLabel>
      <TextField
        id={id}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        disabled={disabled}
        placeholder={t('field.tagsPlaceholder')}
        aria-invalid={tooMany || undefined}
        aria-describedby={helpId}
        className="mt-1"
      />
      {tooMany ? (
        <FieldMessage id={helpId} role="status" className="mt-2">
          {t('field.tagsTooMany', { max: MEMORY_LIMITS.tags, count: tags.length })}
        </FieldMessage>
      ) : (
        <Typography variant="meta" as="p" id={helpId} className="mt-2">
          {t('field.tagsHelp', { max: MEMORY_LIMITS.tags })}
        </Typography>
      )}
    </div>
  )
}
