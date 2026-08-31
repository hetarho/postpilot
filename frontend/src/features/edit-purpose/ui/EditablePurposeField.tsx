import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { remainingChars } from '@/entities/purpose'
import { Button, Editable, FieldLabel, FieldMessage, TextField, Textarea } from '@/shared/ui'

/** One read-first field of a purpose (design-language: read first, edit on request).
 *
 *  The `save` it is given sends only this field, which is what makes the per-field edit safe
 *  under a concurrent edit of a sibling field. A refused save keeps edit mode open with the
 *  draft intact, so nothing typed is lost to a validation message. */
export function EditablePurposeField({
  label,
  value,
  limit,
  multiline = false,
  optional = false,
  placeholder,
  emptyText = '',
  save,
  errorMessage,
  pending,
  className,
}: {
  label: string
  value: string
  limit: number
  multiline?: boolean
  /** Whether an empty value is a valid save. Stated by the caller rather than inferred from
   *  the limit: the limits are build-time overrides, so two of them can coincide and would
   *  make a required field silently accept an empty save. */
  optional?: boolean
  placeholder?: string
  /** What the read view shows when the value is empty — only the optional description has one. */
  emptyText?: string
  save: (next: string) => Promise<unknown>
  errorMessage: string
  pending: boolean
  className?: string
}) {
  const { t } = useTranslation('common')
  const id = useId()
  return (
    <Editable
      className={className}
      editLabel={t('action.editNamed', { name: label })}
      edit={(exit) => (
        <PurposeFieldEditor
          id={id}
          label={label}
          value={value}
          limit={limit}
          multiline={multiline}
          optional={optional}
          placeholder={placeholder}
          save={save}
          errorMessage={errorMessage}
          pending={pending}
          exit={exit}
        />
      )}
    >
      {/* A plain paragraph, not FieldLabel: the read view has no control to label, and a
          `<label>` pointing at nothing is a broken association for a screen reader. */}
      <p className="text-content-tertiary text-xs font-medium">{label}</p>
      <p className="text-content-primary mt-1 text-sm whitespace-pre-wrap">
        {value.trim() || <span className="text-content-tertiary">{emptyText}</span>}
      </p>
    </Editable>
  )
}

function PurposeFieldEditor({
  id,
  label,
  value,
  limit,
  multiline,
  optional,
  placeholder,
  save,
  errorMessage,
  pending,
  exit,
}: {
  id: string
  label: string
  value: string
  limit: number
  multiline: boolean
  optional: boolean
  placeholder?: string
  save: (next: string) => Promise<unknown>
  errorMessage: string
  pending: boolean
  exit: () => void
}) {
  const { t } = useTranslation('common')
  // Seeded once, at the mount this editor gets when edit mode opens. It is deliberately not
  // resynced from `value` afterwards: while someone is typing here, their draft outranks a
  // value that arrives from a refetch or from a sibling field's save.
  const [draft, setDraft] = useState(value)
  const [failed, setFailed] = useState(false)
  const errorId = `${id}-error`

  const left = remainingChars(draft, limit)
  const disabled = pending || left < 0 || (!optional && !draft.trim())

  const commit = async () => {
    if (disabled) return
    try {
      await save(draft)
      setFailed(false)
      exit()
    } catch {
      // Stay open: the message renders below and the draft is still here to fix.
      setFailed(true)
    }
  }

  return (
    <div>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      {multiline ? (
        <Textarea
          id={id}
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
          rows={4}
          autoGrow
          autoFocus
          placeholder={placeholder}
          aria-invalid={failed || undefined}
          aria-describedby={failed ? errorId : undefined}
          className="mt-1"
        />
      ) : (
        <TextField
          id={id}
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
          autoFocus
          autoComplete="off"
          enterKeyHint="done"
          placeholder={placeholder}
          aria-invalid={failed || undefined}
          aria-describedby={failed ? errorId : undefined}
          className="mt-1"
        />
      )}
      <p
        className={
          left < 0 ? 'text-field-error mt-2 text-xs' : 'text-content-tertiary mt-2 text-xs'
        }
      >
        {left < 0 ? t('count.exceeded', { count: -left }) : t('count.remaining', { count: left })}
      </p>
      {failed && errorMessage && (
        <FieldMessage id={errorId} className="mt-2">
          {errorMessage}
        </FieldMessage>
      )}
      <div className="mt-3 flex gap-2">
        <Button onClick={() => void commit()} disabled={disabled} pending={pending}>
          {t('action.save')}
        </Button>
        <Button variant="ghost" onClick={exit} disabled={pending}>
          {t('action.cancel')}
        </Button>
      </div>
    </div>
  )
}
