import { useId, useState, type ChangeEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useVoices, voiceRefLabel, type VoiceRef } from '@/entities/voice'
import { Badge, Dialog, FieldLabel, FieldMessage, Select, Typography } from '@/shared/ui'
import { reassignmentFailureMessage } from '../model/reassignment'

interface PostVoiceSelectProps {
  ownerId: string
  /** The assignment as the editor shows it. */
  value: string
  /** The post's voice as the server reports it — undefined for a draft with no post yet. A
   *  deleted one is listed, disabled, so the field can still say what the post is written in. */
  current?: VoiceRef
  /** Why the assignment cannot change right now, or ''. Shown under the field. */
  blocked?: string
  /** Confirm before applying. An existing post is asked; a draft with no post yet just switches,
   *  since nothing has been learned under either voice. */
  confirm: boolean
  onSelect: (voiceId: string) => Promise<void> | void
  className?: string
}

/** The required voice of a post: a native select wearing the field well (design-language §7),
 *  listing only the voices a post may be assigned to. */
export function PostVoiceSelect({
  ownerId,
  value,
  current,
  blocked = '',
  confirm,
  onSelect,
  className,
}: PostVoiceSelectProps) {
  const { t } = useTranslation(['voices', 'common'])
  const id = useId()
  const hintId = `${id}-hint`
  const errorId = `${id}-error`
  const { active, isPending } = useVoices(ownerId)
  const [target, setTarget] = useState('')
  const [applying, setApplying] = useState(false)
  const [error, setError] = useState('')
  const disabled = Boolean(blocked) || isPending || applying
  // The post's own voice is listed even while the directory is still loading — a select with no
  // option under a post that plainly has a voice reads as broken — and a deleted one stays
  // listed, disabled, so the field can still say what the post is written in.
  const unlisted = current && !active.some((voice) => voice.id === current.id) ? current : undefined
  const selectedVoice = active.find((voice) => voice.id === value) ?? unlisted

  const apply = async (voiceId: string) => {
    setApplying(true)
    setError('')
    try {
      await onSelect(voiceId)
    } catch (cause) {
      setError(reassignmentFailureMessage(cause))
    } finally {
      // Closed on failure too: the message renders under the field, and an open sheet would
      // hide it behind the scrim.
      setTarget('')
      setApplying(false)
    }
  }

  const onChange = (event: ChangeEvent<HTMLSelectElement>) => {
    const next = event.target.value
    if (!next || next === value) return
    if (confirm) setTarget(next)
    else void apply(next)
  }

  const targetName = active.find((voice) => voice.id === target)?.name ?? ''
  const describedBy = [blocked ? hintId : '', error ? errorId : ''].filter(Boolean).join(' ')

  return (
    <div className={className}>
      <div className="flex items-center gap-3">
        <FieldLabel htmlFor={id} className="shrink-0">
          {t('title')}
        </FieldLabel>
        <span className="min-w-0 flex-1">
          <Select
            id={id}
            value={value}
            onChange={onChange}
            disabled={disabled}
            aria-invalid={error ? true : undefined}
            aria-describedby={describedBy || undefined}
          >
            {unlisted && (
              <option value={unlisted.id} disabled={unlisted.deleted}>
                {voiceRefLabel(unlisted)}
              </option>
            )}
            {active.map((voice) => (
              <option key={voice.id} value={voice.id}>
                {voice.name}
              </option>
            ))}
          </Select>
        </span>
        {selectedVoice?.sourceLanguage && (
          <Badge>{t(`contentLanguage.${selectedVoice.sourceLanguage}`, { ns: 'common' })}</Badge>
        )}
      </div>
      {blocked && (
        <Typography variant="label" as="p" id={hintId} role="status" className="mt-2">
          {blocked}
        </Typography>
      )}
      {error && (
        <FieldMessage id={errorId} className="mt-2">
          {error}
        </FieldMessage>
      )}
      <Dialog
        open={target !== ''}
        title={t('assignment.title')}
        confirmLabel={t('assignment.confirm')}
        pending={applying}
        onClose={() => {
          if (!applying) setTarget('')
        }}
        onConfirm={() => {
          if (target) void apply(target)
        }}
      >
        {t('assignment.description', { name: targetName })}
      </Dialog>
    </div>
  )
}
