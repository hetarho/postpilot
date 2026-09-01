import { useId, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import type { Voice } from '@/entities/voice'
import { VOICE_NAME_MAX_CHARS } from '@/shared/config'
import { Button, Editable, FieldLabel, FieldMessage, TextField, Typography } from '@/shared/ui'
import { useRenameVoice } from '../api/useRenameVoice'

/** Read first, rename on request (the `Editable` shape the profile fields use). The caller supplies
 *  the read view — the name is also a link on the directory — and this owns the edit view. Works
 *  for a deleted voice too: renaming a tombstone is how a restore conflict is resolved. */
export function RenameVoiceField({
  ownerId,
  voice,
  children,
}: {
  ownerId: string
  voice: Pick<Voice, 'id' | 'name'>
  children: ReactNode
}) {
  const { t } = useTranslation('voices')
  const rename = useRenameVoice(ownerId)
  const commit = async (exit: () => void, name: string) => {
    try {
      await rename.rename(voice.id, name)
      // Only a successful save leaves edit mode; a rejected one keeps the typed name on screen.
      exit()
    } catch {
      // The mutation's message renders under the field.
    }
  }
  return (
    <Editable
      editLabel={t('rename.aria', { name: voice.name })}
      edit={(exit) => (
        <RenameEditor
          name={voice.name}
          pending={rename.isPending}
          errorMessage={rename.isError ? rename.errorMessage : ''}
          onSave={(name) => commit(exit, name)}
          onCancel={() => {
            rename.reset()
            exit()
          }}
        />
      )}
    >
      {children}
    </Editable>
  )
}

function RenameEditor({
  name,
  pending,
  errorMessage,
  onSave,
  onCancel,
}: {
  name: string
  pending: boolean
  errorMessage: string
  onSave: (name: string) => void
  onCancel: () => void
}) {
  const { t } = useTranslation(['voices', 'common'])
  const id = useId()
  const countId = `${id}-count`
  const errorId = `${id}-error`
  const [draft, setDraft] = useState(name)
  const chars = Array.from(draft.trim()).length
  const exceeded = chars > VOICE_NAME_MAX_CHARS
  const valid = chars > 0 && chars <= VOICE_NAME_MAX_CHARS
  return (
    <form
      onSubmit={(event) => {
        event.preventDefault()
        if (valid && !pending) onSave(draft.trim())
      }}
    >
      <FieldLabel htmlFor={id}>{t('rename.label', { ns: 'voices' })}</FieldLabel>
      <TextField
        id={id}
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
        maxLength={VOICE_NAME_MAX_CHARS * 2}
        autoComplete="off"
        autoCapitalize="off"
        autoCorrect="off"
        enterKeyHint="done"
        autoFocus
        aria-invalid={exceeded || Boolean(errorMessage) || undefined}
        aria-describedby={`${countId}${errorMessage ? ` ${errorId}` : ''}`}
        className="mt-1"
      />
      {exceeded ? (
        <FieldMessage id={countId} role="status" className="mt-2">
          {t('count.exceeded', {
            ns: 'common',
            count: chars - VOICE_NAME_MAX_CHARS,
          })}
        </FieldMessage>
      ) : (
        <Typography variant="meta" as="p" id={countId} className="mt-2">
          {t('rename.count', { ns: 'voices', count: chars, max: VOICE_NAME_MAX_CHARS })}
        </Typography>
      )}
      {errorMessage && (
        <FieldMessage id={errorId} className="mt-2">
          {errorMessage}
        </FieldMessage>
      )}
      <div className="mt-2 flex flex-wrap gap-2">
        <Button type="submit" variant="secondary" disabled={!valid} pending={pending}>
          {t('action.save', { ns: 'common' })}
        </Button>
        <Button variant="ghost" disabled={pending} onClick={onCancel}>
          {t('action.cancel', { ns: 'common' })}
        </Button>
      </div>
    </form>
  )
}
