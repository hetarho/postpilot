import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  NO_VOICE_VALUE,
  noVoiceLabel,
  useVoices,
  voiceRefLabel,
  type VoiceRef,
} from '@/entities/voice'
import {
  Dialog,
  FieldLabel,
  FieldMessage,
  Listbox,
  Typography,
  type ListboxOption,
} from '@/shared/ui'
import { reassignmentFailureMessage } from '../model/reassignment'

/** The option that opens the create sheet instead of choosing anything. A voice id is minted by
 *  the server and never takes this shape, and '' is already 말투 없음. */
const CREATE_VOICE_VALUE = '__create_voice__'

interface PostVoiceSelectProps {
  ownerId: string
  /** The assignment as the editor shows it; '' is 말투 없음. */
  value: string
  /** The post's voice as the server reports it — undefined for a draft with no post yet and for
   *  a post with 말투 없음. A deleted one is listed, disabled, so the field can still say what the
   *  post is written in. */
  current?: VoiceRef
  /** Why the assignment cannot change right now, or ''. Shown under the field. */
  blocked?: string
  /** Confirm before applying. An existing post is asked; a draft with no post yet just switches,
   *  since nothing has been learned under either voice. */
  confirm: boolean
  onSelect: (voiceId: string) => Promise<void> | void
  /** `새 말투 만들기` was picked (VOICE-53). The caller opens the create sheet — a feature may not
   *  render a sibling feature (ARCH-13) — and the assignment stays what it was. */
  onCreateVoice: () => void
  /** Off for a reason the caller states elsewhere, so this field adds none of its own. */
  disabled?: boolean
  className?: string
}

/** The post's voice: an app-drawn listbox wearing the field well (THEME-29), offering 말투 없음,
 *  the made voices, each one not yet made (listed but not choosable) and `새 말투 만들기`
 *  (POST-101).
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
  onCreateVoice,
  disabled: off = false,
  className,
}: PostVoiceSelectProps) {
  const { t } = useTranslation(['voices', 'common'])
  const id = useId()
  const labelId = `${id}-label`
  const hintId = `${id}-hint`
  const errorId = `${id}-error`
  const { active, isPending } = useVoices(ownerId)
  // The voice a confirmation is open for; undefined while none is, since '' is a real target.
  const [target, setTarget] = useState<string>()
  const [applying, setApplying] = useState(false)
  const [error, setError] = useState('')
  const disabled = off || Boolean(blocked) || isPending || applying
  // The post's own voice is listed even while the directory is still loading — a select showing
  // 말투 없음 under a post that plainly has a voice reads as if the assignment were lost — and a
  // deleted one stays listed, disabled, so the field can still say what the post is written in.
  const unlisted = current && !active.some((voice) => voice.id === current.id) ? current : undefined
  // A voice not yet made, as the list shows it: disabled, and saying why (POST-101).
  const makingLabel = (name: string) => t('picker.making', { name })

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
      setTarget(undefined)
      setApplying(false)
    }
  }

  const onChange = (next: string) => {
    // Not a choice: the sheet opens over the field, and the listbox, which is controlled, keeps
    // showing the assignment the post still has.
    if (next === CREATE_VOICE_VALUE) {
      onCreateVoice()
      return
    }
    if (next === value) return
    if (confirm) setTarget(next)
    else void apply(next)
  }

  const options: ListboxOption<string>[] = [
    { value: NO_VOICE_VALUE, label: noVoiceLabel() },
    ...(unlisted
      ? [
          {
            value: unlisted.id,
            label:
              unlisted.deleted || unlisted.made
                ? voiceRefLabel(unlisted)
                : makingLabel(unlisted.name),
            disabled: unlisted.deleted || !unlisted.made,
          },
        ]
      : []),
    ...active
      .filter((voice) => voice.made)
      .map((voice) => ({ value: voice.id, label: voice.name })),
    // Listed so the owner can see the voice exists and is on its way; choosable once it is made
    // (VOICE-32). No readiness share yet: the directory does not carry one.
    ...active
      .filter((voice) => !voice.made)
      .map((voice) => ({ value: voice.id, label: makingLabel(voice.name), disabled: true })),
    { value: CREATE_VOICE_VALUE, label: t('picker.create') },
  ]

  const targetName =
    target === NO_VOICE_VALUE
      ? noVoiceLabel()
      : (active.find((voice) => voice.id === target)?.name ?? '')
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
        open={target !== undefined}
        title={t('assignment.title')}
        confirmLabel={t('assignment.confirm')}
        pending={applying}
        onClose={() => {
          if (!applying) setTarget(undefined)
        }}
        onConfirm={() => {
          if (target !== undefined) void apply(target)
        }}
      >
        {t('assignment.description', { name: targetName })}
      </Dialog>
    </div>
  )
}
