import { useId, useState, type FormEvent } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { VOICE_NAME_MAX_CHARS, useCreateVoice } from '@/entities/voice'
import { Button, FieldLabel, FieldMessage, Sheet, TextField, Typography } from '@/shared/ui'

type CreateVoiceSheetProps = { ownerId: string; className?: string } & (
  | { open?: undefined; onOpenChange?: undefined }
  | {
      /** Controlled: the caller owns the open state and renders its own way in — the post voice
       *  picker's `새 말투 만들기` option (VOICE-53) — so no button is drawn here. */
      open: boolean
      onOpenChange: (open: boolean) => void
    }
)

/** The directory's one committing action and the overlay it opens. The trigger is rendered here
 *  rather than by the page so the open state stays with the form it opens, the way `Popover`
 *  keeps its own — unless a caller that already has a way in passes `open`. */
export function CreateVoiceSheet({
  ownerId,
  className,
  open,
  onOpenChange,
}: CreateVoiceSheetProps) {
  const { t } = useTranslation('voices')
  const [ownOpen, setOwnOpen] = useState(false)
  const controlled = open !== undefined
  const shown = open ?? ownOpen
  const setShown = onOpenChange ?? setOwnOpen
  return (
    <>
      {!controlled && (
        <Button variant="cta" className={className} onClick={() => setOwnOpen(true)}>
          {t('create.open')}
        </Button>
      )}
      {shown && <CreateVoicePanel ownerId={ownerId} onClose={() => setShown(false)} />}
    </>
  )
}

/** Mounted only while the sheet is open, so the field starts blank on each visit and a failed
 *  attempt's message does not greet the next one. The name is all a voice is created with
 *  (VOICE-10, VOICE-53). */
function CreateVoicePanel({ ownerId, onClose }: { ownerId: string; onClose: () => void }) {
  const { t } = useTranslation(['voices', 'common'])
  const navigate = useNavigate()
  const id = useId()
  const titleId = `${id}-title`
  const nameId = `${id}-name`
  const countId = `${id}-count`
  const errorId = `${id}-error`
  const [name, setName] = useState('')
  const create = useCreateVoice(ownerId)

  // Counted the way the server counts (Unicode scalar values), so the field and the rule agree
  // on text made of Hangul and emoji alike.
  const chars = Array.from(name.trim()).length
  const exceeded = chars > VOICE_NAME_MAX_CHARS
  const disabled = chars === 0 || exceeded || create.isPending

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (disabled) return
    try {
      const response = await create.create({ name: name.trim() })
      onClose()
      if (response.voice) {
        await navigate({ to: '/voices/$voiceId', params: { voiceId: response.voice.id } })
      }
    } catch {
      // The mutation's message renders under the field, inside the still-open sheet.
    }
  }

  return (
    <Sheet open labelledBy={titleId} onClose={create.isPending ? () => {} : onClose}>
      <Typography variant="title" as="h2" id={titleId}>
        {t('create.title', { ns: 'voices' })}
      </Typography>
      <form onSubmit={(event) => void submit(event)} className="mt-4">
        <FieldLabel htmlFor={nameId}>{t('create.name', { ns: 'voices' })}</FieldLabel>
        <TextField
          id={nameId}
          value={name}
          onChange={(event) => setName(event.target.value)}
          placeholder={t('create.placeholder', { ns: 'voices' })}
          maxLength={VOICE_NAME_MAX_CHARS * 2}
          autoComplete="off"
          autoCapitalize="off"
          autoCorrect="off"
          enterKeyHint="done"
          autoFocus
          aria-invalid={exceeded || create.isError || undefined}
          aria-describedby={create.isError ? `${countId} ${errorId}` : countId}
          className="mt-1"
        />
        {exceeded ? (
          <FieldMessage id={countId} role="status" className="mt-2">
            {t('count.exceeded', { ns: 'common', count: chars - VOICE_NAME_MAX_CHARS })}
          </FieldMessage>
        ) : (
          <Typography variant="meta" as="p" id={countId} className="mt-2">
            {t('create.count', { ns: 'voices', count: chars, max: VOICE_NAME_MAX_CHARS })}
          </Typography>
        )}
        {create.isError && (
          <FieldMessage id={errorId} className="mt-2">
            {create.errorMessage}
          </FieldMessage>
        )}
        {/* In flow after the field, NOT in the sheet's pinned footer: the panel is anchored to
            the layout viewport, which the software keyboard does not resize, so a pinned footer
            sits behind the keyboard exactly while the field is being typed into (THEME-31). */}
        <div className="mt-6 flex flex-wrap justify-end gap-2">
          <Button variant="ghost" disabled={create.isPending} onClick={onClose}>
            {t('action.cancel', { ns: 'common' })}
          </Button>
          <Button type="submit" variant="cta" disabled={disabled} pending={create.isPending}>
            {t('create.submit', { ns: 'voices' })}
          </Button>
        </div>
      </form>
    </Sheet>
  )
}
