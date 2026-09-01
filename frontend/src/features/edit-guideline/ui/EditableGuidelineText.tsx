import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { remainingGuidelineChars } from '@/entities/guideline'
import { Button, Editable, FieldLabel, FieldMessage, Textarea, Typography } from '@/shared/ui'

/** A guideline's text, read first and edited on request (design-language: read first).
 *
 *  `save` sends only the text, which is what makes this edit safe under a concurrent scope
 *  replacement. A refused save keeps edit mode open with the draft intact, so nothing typed is
 *  lost to a validation message. */
export function EditableGuidelineText({
  value,
  save,
  errorMessage,
  pending,
  className,
}: {
  value: string
  save: (next: string) => Promise<unknown>
  errorMessage: string
  pending: boolean
  className?: string
}) {
  const { t } = useTranslation(['guidelines', 'common'])
  const id = useId()
  const label = t('edit.text', { ns: 'guidelines' })
  return (
    <Editable
      className={className}
      editLabel={t('action.editNamed', { ns: 'common', name: label })}
      edit={(exit) => (
        <GuidelineTextEditor
          id={id}
          label={label}
          value={value}
          save={save}
          errorMessage={errorMessage}
          pending={pending}
          exit={exit}
        />
      )}
    >
      {/* A plain paragraph, not FieldLabel: the read view has no control to label, and a
          `<label>` pointing at nothing is a broken association for a screen reader. */}
      <Typography variant="body" className="text-content-primary whitespace-pre-wrap">
        {value}
      </Typography>
    </Editable>
  )
}

function GuidelineTextEditor({
  id,
  label,
  value,
  save,
  errorMessage,
  pending,
  exit,
}: {
  id: string
  label: string
  value: string
  save: (next: string) => Promise<unknown>
  errorMessage: string
  pending: boolean
  exit: () => void
}) {
  const { t } = useTranslation('common')
  // Seeded once, at the mount edit mode gives this editor. Deliberately not resynced from
  // `value`: while someone is typing here, their draft outranks a value arriving from a refetch.
  const [draft, setDraft] = useState(value)
  const [failed, setFailed] = useState(false)
  const countId = `${id}-count`
  const errorId = `${id}-error`

  const left = remainingGuidelineChars(draft)
  const exceeded = left < 0
  const showSaveError = failed && Boolean(errorMessage)
  const disabled = pending || left < 0 || !draft.trim()

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
      <Textarea
        id={id}
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
        rows={3}
        autoGrow
        autoFocus
        aria-invalid={exceeded || failed || undefined}
        aria-describedby={`${countId}${showSaveError ? ` ${errorId}` : ''}`}
        className="mt-1"
      />
      {exceeded ? (
        <FieldMessage id={countId} role="status" className="mt-2">
          {t('count.exceeded', { count: -left })}
        </FieldMessage>
      ) : (
        <Typography variant="meta" as="p" id={countId} className="mt-2">
          {t('count.remaining', { count: left })}
        </Typography>
      )}
      {showSaveError && (
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
