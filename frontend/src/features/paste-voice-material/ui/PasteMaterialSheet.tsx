import { useId, useRef, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useAddVoiceSample } from '@/entities/voice'
import {
  Button,
  FieldLabel,
  FieldMessage,
  Sheet,
  Textarea,
  TextField,
  Typography,
} from '@/shared/ui'
import { VOICE_SAMPLE_MIN_CHARS } from '../config'

/** `글 붙여넣기` and the sheet it opens (VOICE-20, VOICE-64): a post the owner wrote by hand,
 *  pasted as a 학습 글. The sheet stays open after each post, blank for the next one, until the
 *  owner closes it (VOICE-65). It needs no model and starts nothing. */
export function PasteMaterialSheet({
  ownerId,
  voiceId,
  disabled = false,
}: {
  ownerId: string
  voiceId: string
  disabled?: boolean
}) {
  const { t } = useTranslation('voices')
  const [open, setOpen] = useState(false)
  return (
    <>
      <Button variant="secondary" disabled={disabled} onClick={() => setOpen(true)}>
        {t('paste.open')}
      </Button>
      {open && <PastePanel ownerId={ownerId} voiceId={voiceId} onClose={() => setOpen(false)} />}
    </>
  )
}

/** Mounted only while open, so each visit starts blank. */
function PastePanel({
  ownerId,
  voiceId,
  onClose,
}: {
  ownerId: string
  voiceId: string
  onClose: () => void
}) {
  const { t } = useTranslation(['voices', 'common'])
  const id = useId()
  const titleId = `${id}-title`
  const labelId = `${id}-label`
  const bodyId = `${id}-body`
  const hintId = `${id}-hint`
  const errorId = `${id}-error`
  const labelField = useRef<HTMLInputElement>(null)
  const [label, setLabel] = useState('')
  const [body, setBody] = useState('')
  const [added, setAdded] = useState(0)
  const [saved, setSaved] = useState(false)
  const add = useAddVoiceSample(ownerId, voiceId)
  const chars = Array.from(body.trim()).length
  const tooShort = chars < VOICE_SAMPLE_MIN_CHARS
  const disabled = tooShort || add.isPending

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (disabled) return
    try {
      await add.add(label, body)
      setLabel('')
      setBody('')
      setAdded((count) => count + 1)
      setSaved(true)
      labelField.current?.focus()
    } catch {
      // The mutation's message renders under the body, inside the still-open sheet.
    }
  }

  return (
    <Sheet open labelledBy={titleId} onClose={add.isPending ? () => {} : onClose}>
      <Typography variant="title" as="h2" id={titleId}>
        {t('paste.title', { ns: 'voices' })}
      </Typography>
      {/* Present at rest, so the post it later names is announced (VOICE-65). */}
      <Typography variant="meta" as="p" role="status" className="mt-1">
        {saved ? t('paste.added', { ns: 'voices' }) : ''}
      </Typography>
      <form onSubmit={(event) => void submit(event)} className="mt-4 space-y-4">
        <div>
          <FieldLabel htmlFor={labelId}>{t('paste.label', { ns: 'voices' })}</FieldLabel>
          <TextField
            ref={labelField}
            id={labelId}
            value={label}
            onChange={(event) => {
              setSaved(false)
              setLabel(event.target.value)
            }}
            placeholder={t('paste.labelPlaceholder', { ns: 'voices' })}
            autoComplete="off"
            autoCapitalize="off"
            autoCorrect="off"
            enterKeyHint="next"
            className="mt-1"
          />
        </div>
        <div>
          <FieldLabel htmlFor={bodyId}>{t('paste.body', { ns: 'voices' })}</FieldLabel>
          <Textarea
            id={bodyId}
            value={body}
            onChange={(event) => {
              setSaved(false)
              setBody(event.target.value)
            }}
            placeholder={t('paste.bodyPlaceholder', { ns: 'voices' })}
            rows={6}
            autoGrow
            aria-invalid={add.isError || undefined}
            aria-describedby={add.isError ? `${hintId} ${errorId}` : hintId}
            className="max-h-field mt-1"
          />
          {/* Under the field: it is the only explanation for the disabled button, and above the
              textarea it scrolls away as soon as the text grows (THEME-24). */}
          <div id={hintId} className="mt-2 flex flex-wrap items-baseline gap-x-3 gap-y-1">
            <Typography variant="meta" className="shrink-0">
              {t('paste.count', { ns: 'voices', count: chars, min: VOICE_SAMPLE_MIN_CHARS })}
            </Typography>
            {tooShort && chars > 0 && (
              <Typography variant="label" className="min-w-0">
                {t('paste.remaining', { ns: 'voices', count: VOICE_SAMPLE_MIN_CHARS - chars })}
              </Typography>
            )}
          </div>
          {add.isError && (
            <FieldMessage id={errorId} className="mt-2">
              {add.errorMessage}
            </FieldMessage>
          )}
        </div>
        {/* In flow after the fields, not in a pinned footer the software keyboard covers
            (THEME-31). */}
        <div className="flex flex-wrap justify-end gap-2">
          <Button variant="ghost" disabled={add.isPending} onClick={onClose}>
            {added > 0 ? t('action.close', { ns: 'common' }) : t('action.cancel', { ns: 'common' })}
          </Button>
          <Button type="submit" variant="cta" disabled={disabled} pending={add.isPending}>
            {t('paste.submit', { ns: 'voices' })}
          </Button>
        </div>
      </form>
    </Sheet>
  )
}
