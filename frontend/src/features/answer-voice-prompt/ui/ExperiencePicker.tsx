import { useId, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import {
  VOICE_EXPERIENCE_NAME_MAX_CHARS,
  type VoiceExperienceKindInfo,
  type VoiceExperienceKindName,
} from '@/entities/voice'
import { Button, ChipToggle, FieldLabel, FieldMessage, TextField, Typography } from '@/shared/ui'

/** The learning chat's first turn (VOICE-78): one kind of the six and an optional name for the
 *  recent experience the owner brings. Sending starts the experience; `생각나는 게 없어요` leaves
 *  for the imagined situations when the caller offers it. */
export function ExperiencePicker({
  kinds,
  pending,
  errorMessage,
  onStart,
  onNothing,
}: {
  kinds: readonly VoiceExperienceKindInfo[]
  pending: boolean
  errorMessage?: string
  onStart: (kind: VoiceExperienceKindName, name: string) => void
  /** Absent once the owner has an experience, since the fallback belongs to the first turn. */
  onNothing?: () => void
}) {
  const { t } = useTranslation('voices')
  const id = useId()
  const [kind, setKind] = useState<VoiceExperienceKindName>()
  const [name, setName] = useState('')
  // Counted the way the server counts, in Unicode scalar values (VOICE-78).
  const chars = Array.from(name.trim()).length
  const exceeded = chars > VOICE_EXPERIENCE_NAME_MAX_CHARS
  const disabled = kind === undefined || exceeded || pending
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!disabled) onStart(kind, name.trim())
  }
  return (
    <form
      onSubmit={submit}
      className="bg-surface-base pb-safe-b sticky bottom-0 z-10 min-w-0 space-y-3 pt-3"
    >
      <div role="group" aria-labelledby={`${id}-kind`} className="space-y-2">
        <Typography variant="label" id={`${id}-kind`}>
          {t('prompts.chat.experience.kindLabel')}
        </Typography>
        <div className="flex flex-wrap gap-2">
          {kinds.map((info) => (
            <ChipToggle
              key={info.kind}
              pressed={kind === info.kind}
              disabled={pending}
              onClick={() => setKind(info.kind)}
            >
              {info.label}
            </ChipToggle>
          ))}
        </div>
      </div>
      <div>
        <FieldLabel htmlFor={`${id}-name`}>{t('prompts.chat.experience.nameLabel')}</FieldLabel>
        <TextField
          id={`${id}-name`}
          value={name}
          disabled={pending}
          onChange={(event) => setName(event.target.value)}
          placeholder={t('prompts.chat.experience.namePlaceholder')}
          maxLength={VOICE_EXPERIENCE_NAME_MAX_CHARS * 2}
          autoComplete="off"
          autoCapitalize="off"
          autoCorrect="off"
          enterKeyHint="send"
          aria-invalid={exceeded || undefined}
          aria-describedby={`${id}-count`}
          className="mt-1"
        />
        {exceeded ? (
          <FieldMessage id={`${id}-count`} role="status" className="mt-2">
            {t('count.exceeded', { ns: 'common', count: chars - VOICE_EXPERIENCE_NAME_MAX_CHARS })}
          </FieldMessage>
        ) : (
          <Typography variant="meta" as="p" id={`${id}-count`} className="mt-2">
            {t('prompts.chat.experience.count', {
              current: chars,
              max: VOICE_EXPERIENCE_NAME_MAX_CHARS,
            })}
          </Typography>
        )}
      </div>
      {errorMessage && <FieldMessage role="alert">{errorMessage}</FieldMessage>}
      <div className="flex flex-wrap items-center justify-end gap-2">
        {onNothing && (
          <Button variant="ghost" disabled={pending} onClick={onNothing}>
            {t('prompts.chat.experience.nothing')}
          </Button>
        )}
        <Button type="submit" variant="cta" pending={pending} disabled={disabled}>
          {t('prompts.chat.send')}
        </Button>
      </div>
    </form>
  )
}
