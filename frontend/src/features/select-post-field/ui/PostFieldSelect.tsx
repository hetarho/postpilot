import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  BLOG_FIELD_IDS,
  NO_BLOG_FIELD,
  blogFieldLabelKey,
  type BlogFieldChoice,
} from '@/entities/blog-field'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import { FieldLabel, FieldMessage, Listbox, Typography, type ListboxOption } from '@/shared/ui'

interface PostFieldSelectProps {
  /** The 분야 as the editor shows it; '' is 없음. */
  value: BlogFieldChoice
  /** Off for a reason the caller states elsewhere, so this field adds none of its own. */
  disabled?: boolean
  /** Saves the choice: through the draft queue on a saved post, into local state on /posts/new. A
   *  rejection is shown under the field; taking the choice back is the caller's. */
  onSelect: (choice: BlogFieldChoice) => Promise<void> | void
  className?: string
}

/** The post's optional 분야 (POST-82): an app-drawn listbox wearing the field well like ①'s other
 *  fields (design-language §7), defaulting to 없음. The value is read when a run is enqueued, so
 *  it stays usable while a job runs — the job keeps the 분야 it started with. */
export function PostFieldSelect({
  value,
  disabled = false,
  onSelect,
  className,
}: PostFieldSelectProps) {
  const { t } = useTranslation('posts')
  const id = useId()
  const labelId = `${id}-label`
  const hintId = `${id}-hint`
  const errorId = `${id}-error`
  const [pending, setPending] = useState(false)
  const [failure, setFailure] = useState<AppFailure>()

  const options: ListboxOption<BlogFieldChoice>[] = [
    { value: NO_BLOG_FIELD, label: t(blogFieldLabelKey(NO_BLOG_FIELD)) },
    ...BLOG_FIELD_IDS.map((field) => ({ value: field, label: t(blogFieldLabelKey(field)) })),
  ]

  const select = async (next: BlogFieldChoice) => {
    if (next === value) return
    setFailure(undefined)
    setPending(true)
    try {
      await onSelect(next)
    } catch (cause) {
      setFailure(appFailureFromConnect(cause))
    } finally {
      setPending(false)
    }
  }

  return (
    <div className={className}>
      <FieldLabel id={labelId} htmlFor={id}>
        {t('postField.label')}
      </FieldLabel>
      <Listbox<BlogFieldChoice>
        id={id}
        aria-labelledby={labelId}
        value={value}
        options={options}
        disabled={disabled || pending}
        aria-invalid={failure ? true : undefined}
        aria-describedby={`${hintId}${failure ? ` ${errorId}` : ''}`}
        onChange={(next) => void select(next)}
        className="mt-1"
      />
      <Typography variant="body" as="p" id={hintId} className="text-content-secondary mt-2">
        {t('postField.help')}
      </Typography>
      {failure && (
        <FieldMessage id={errorId} className="mt-2">
          {formatAppFailure(failure)}
        </FieldMessage>
      )}
    </div>
  )
}
