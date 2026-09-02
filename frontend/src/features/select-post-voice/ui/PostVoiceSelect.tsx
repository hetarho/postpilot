import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useVoices, voiceRefLabel, type VoiceRef } from '@/entities/voice'
import {
  Badge,
  Dialog,
  FieldLabel,
  FieldMessage,
  Listbox,
  Typography,
  type ListboxOption,
} from '@/shared/ui'
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

/** The required voice of a post: an app-drawn listbox wearing the field well (design-language §7),
 *  listing only the voices a post may be assigned to.
 *
 *  It rides the editor dock's own row, sharing it with the 용도 field and the writing brief's
 *  glyph, so it carries NO visible label: three columns across a 360px screen leaves each field
 *  about 140px, and a '말투' caption would spend a third of that saying what the trigger's own
 *  value already says. The label element stays, `sr-only`, so the control is still announced as
 *  '말투 <값>'. */
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
  const labelId = `${id}-label`
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

  const onChange = (next: string) => {
    if (!next || next === value) return
    if (confirm) setTarget(next)
    else void apply(next)
  }

  const options: ListboxOption<string>[] = [
    ...(unlisted
      ? [{ value: unlisted.id, label: voiceRefLabel(unlisted), disabled: unlisted.deleted }]
      : []),
    ...active.map((voice) => ({ value: voice.id, label: voice.name })),
  ]

  const targetName = active.find((voice) => voice.id === target)?.name ?? ''
  const describedBy = [blocked ? hintId : '', error ? errorId : ''].filter(Boolean).join(' ')

  return (
    <div className={className}>
      <div className="flex items-center gap-2">
        <FieldLabel id={labelId} htmlFor={id} className="sr-only">
          {t('title')}
        </FieldLabel>
        <span className="min-w-0 flex-1">
          <Listbox
            id={id}
            aria-labelledby={labelId}
            value={value}
            options={options}
            onChange={onChange}
            disabled={disabled}
            aria-invalid={error ? true : undefined}
            aria-describedby={describedBy || undefined}
          />
        </span>
        {/* Provenance, not a decision the row is for, and it costs a third of the trigger's width
            on a phone. A voice written in the wrong language is called out by `VoiceWarning` and
            by the 글 언어 field's own mismatch line, neither of which this chip is standing in
            for, so it appears only where there is room for it. */}
        {selectedVoice?.sourceLanguage && (
          <Badge className="hidden sm:inline-flex">
            {t(`contentLanguage.${selectedVoice.sourceLanguage}`, { ns: 'common' })}
          </Badge>
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
