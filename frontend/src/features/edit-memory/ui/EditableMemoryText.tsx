import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { remainingMemoryChars } from '@/entities/memory'
import { Button, Editable, FieldLabel, FieldMessage, Textarea, Typography } from '@/shared/ui'

/** A memory's text, read first and edited on request (design-language: read first).
 *
 *  `save` sends only the text, which is what makes this edit safe under a concurrent facet
 *  change. A refused save keeps edit mode open with the draft intact — the refusal a text edit
 *  can meet is a text the account already holds, and fixing it means changing what is typed. */
export function EditableMemoryText({
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
  const { t } = useTranslation(['memories', 'common'])
  const id = useId()
  const label = t('edit.text', { ns: 'memories' })
  return (
    <Editable
      className={className}
      editLabel={t('action.editNamed', { ns: 'common', name: label })}
      edit={(exit) => (
        <MemoryTextEditor
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
      <Typography variant="body" className="text-content-primary whitespace-pre-wrap">
        {value}
      </Typography>
    </Editable>
  )
}

function MemoryTextEditor({
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

  const left = remainingMemoryChars(draft)
  const exceeded = left < 0
  const showSaveError = failed && Boolean(errorMessage)
  const disabled = pending || exceeded || !draft.trim()

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
