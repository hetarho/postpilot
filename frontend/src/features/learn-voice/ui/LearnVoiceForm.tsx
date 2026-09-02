import { useId, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useStageSelection } from '@/entities/model-catalog'
import { type VoiceProfile, useAddVoiceSample } from '@/entities/voice'
import { VOICE_SAMPLE_MIN_CHARS } from '@/shared/config'
import {
  Button,
  Dialog,
  FieldLabel,
  FieldMessage,
  Notice,
  Textarea,
  TextField,
  Typography,
} from '@/shared/ui'

interface LearnVoiceFormProps {
  ownerId: string
  voiceId: string
  profile: VoiceProfile
  onStarted: (jobId: string) => void
  /** Why this voice cannot take a sample right now (a deleted voice), or ''. The server refuses
   *  it either way; this says so before the paste. */
  blocked?: string
}

export function LearnVoiceForm({
  ownerId,
  voiceId,
  profile,
  onStarted,
  blocked = '',
}: LearnVoiceFormProps) {
  const { t } = useTranslation(['voices', 'common'])
  const labelId = useId()
  const bodyId = useId()
  const bodyErrorId = `${bodyId}-error`
  const bodyHintId = `${bodyId}-hint`
  const [label, setLabel] = useState('')
  const [body, setBody] = useState('')
  const [confirmOverwrite, setConfirmOverwrite] = useState(false)
  const { selected, isPending: modelPending } = useStageSelection('analyze')
  const addSample = useAddVoiceSample(ownerId, voiceId)
  const chars = Array.from(body.trim()).length
  const tooShort = chars < VOICE_SAMPLE_MIN_CHARS
  const disabled = Boolean(blocked) || tooShort || !selected || modelPending || addSample.isPending

  const learn = async () => {
    if (!selected) return
    const submittedLabel = label
    const submittedBody = body
    try {
      const response = await addSample.add(submittedLabel, submittedBody, selected)
      setLabel((current) => (current === submittedLabel ? '' : current))
      setBody((current) => (current === submittedBody ? '' : current))
      onStarted(response.jobId)
    } catch {
      // The mutation exposes the server's raw message below.
    } finally {
      // Closed on failure too: the server's message renders under the field, and an open sheet
      // would hide it behind the scrim.
      setConfirmOverwrite(false)
    }
  }

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (disabled || !selected) return
    // Re-analysis replaces an analysis the voice already published, so it is confirmed —
    // through the sheet, never `window.confirm`: a mobile browser offers to suppress that dialog
    // after a repeat, and from then on it returns false and 학습 is a silent no-op (§7). The sheet
    // also renders inside the page, so the keyboard does not slam shut and reflow the viewport.
    // The gate is the PUBLISHED VERSION now: the free-text styleguide it used to read is gone
    // (change 16), and a voice with a version is exactly a voice with an analysis to replace.
    if (profile.structured.version > 0n) {
      setConfirmOverwrite(true)
      return
    }
    void learn()
  }

  return (
    <form onSubmit={submit} className="mt-5 space-y-4">
      <div>
        <FieldLabel htmlFor={labelId}>{t('learn.label')}</FieldLabel>
        <TextField
          id={labelId}
          value={label}
          onChange={(event) => setLabel(event.target.value)}
          placeholder={t('learn.labelPlaceholder')}
          // A short free-text name, so nothing to autofill and nothing to auto-capitalise. The
          // return key says 다음 rather than the 이동 that implicit submission renders — that key
          // does nothing until the body below is long enough, with no way to say so (§7).
          autoComplete="off"
          autoCapitalize="off"
          autoCorrect="off"
          enterKeyHint="next"
          className="mt-1"
        />
      </div>
      <div>
        <FieldLabel htmlFor={bodyId}>{t('learn.body')}</FieldLabel>
        <Typography variant="label" as="p" className="mt-1">
          {t('learn.declaredLanguage', {
            language: t(`contentLanguage.${profile.voice.sourceLanguage}`, { ns: 'common' }),
          })}
        </Typography>
        <Textarea
          id={bodyId}
          value={body}
          onChange={(event) => setBody(event.target.value)}
          placeholder={t('learn.bodyPlaceholder')}
          // A pasted article is always longer than any fixed box; growing keeps the page the one
          // scroller and leaves the CTA directly under the end of the text (§4.4).
          rows={6}
          autoGrow
          aria-invalid={addSample.isError || undefined}
          aria-describedby={addSample.isError ? `${bodyHintId} ${bodyErrorId}` : bodyHintId}
          className="max-h-field mt-1"
        />
        {/* Under the field, not above it. This is the only explanation for the disabled 학습
            button, and above the textarea it scrolls off the top as soon as the user is typing
            past the first few lines — a validation message behind the keyboard has not been
            shown (§4.3). */}
        <div id={bodyHintId} className="mt-2 flex flex-wrap items-baseline gap-x-3 gap-y-1">
          <Typography variant="meta" className="shrink-0">
            {t('learn.count', { count: chars, min: VOICE_SAMPLE_MIN_CHARS })}
          </Typography>
          {tooShort && (
            <Typography variant="label" className="min-w-0">
              {t('learn.remaining', { count: VOICE_SAMPLE_MIN_CHARS - chars })}
            </Typography>
          )}
        </div>
      </div>

      {blocked && (
        <Notice tone="warning" role="status">
          {blocked}
        </Notice>
      )}
      {!blocked && !modelPending && !selected && (
        <Notice tone="warning" role="status">
          {t('learn.selectModel')}
        </Notice>
      )}
      {addSample.isError && <FieldMessage id={bodyErrorId}>{addSample.errorMessage}</FieldMessage>}

      <Button
        type="submit"
        variant="cta"
        disabled={disabled}
        pending={addSample.isPending}
        aria-describedby={tooShort ? bodyHintId : undefined}
        className="w-full sm:w-auto"
      >
        {t('learn.action')}
      </Button>
      <span role="status" className="sr-only">
        {addSample.isPending ? t('learn.pending') : ''}
      </span>

      <Dialog
        open={confirmOverwrite}
        title={t('learn.confirmTitle')}
        confirmLabel={t('learn.confirm')}
        pending={addSample.isPending}
        onClose={() => setConfirmOverwrite(false)}
        onConfirm={() => void learn()}
      >
        {t('learn.confirmDescription')}
      </Dialog>
    </form>
  )
}
