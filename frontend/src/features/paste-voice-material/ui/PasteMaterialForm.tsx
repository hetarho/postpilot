import { useEffect, useId, useRef, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useAddVoiceSample } from '@/entities/voice'
import { Button, FieldLabel, FieldMessage, Textarea, TextField, Typography } from '@/shared/ui'
import { VOICE_SAMPLE_MIN_CHARS } from '../config'
export interface PasteMaterialDraft {
  label: string
  body: string
}
export interface PasteMaterialFormProps {
  ownerId: string
  voiceId: string
  value?: PasteMaterialDraft
  onChange?: (draft: PasteMaterialDraft) => void
  onSaved?: () => void
  onBusyChange?: (busy: boolean) => void
  onBack?: () => void
  active?: boolean
  focusAfterSave?: boolean
}
export function PasteMaterialForm(props: PasteMaterialFormProps) {
  return <ScopedPaste key={`${props.ownerId}:${props.voiceId}`} {...props} />
}
function ScopedPaste({
  ownerId,
  voiceId,
  value,
  onChange,
  onSaved,
  onBusyChange,
  onBack,
  active = true,
  focusAfterSave = false,
}: PasteMaterialFormProps) {
  const { t } = useTranslation(['voices', 'common'])
  const id = useId()
  const titleField = useRef<HTMLInputElement>(null)
  const [local, setLocal] = useState<PasteMaterialDraft>({ label: '', body: '' })
  const draft = value ?? local
  const [attempted, setAttempted] = useState(false)
  const [saved, setSaved] = useState(false)
  const [confirmationFailed, setConfirmationFailed] = useState(false)
  const add = useAddVoiceSample(ownerId, voiceId)
  useEffect(() => {
    if (
      saved &&
      focusAfterSave &&
      active &&
      !add.isPending &&
      !window.matchMedia?.('(pointer: coarse)').matches
    )
      titleField.current?.focus()
  }, [saved, focusAfterSave, active, add.isPending])

  const lock = useRef(false),
    mounted = useRef(true)
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])
  const change = (next: PasteMaterialDraft) => {
    setLocal(next)
    onChange?.(next)
    setSaved(false)
    setAttempted(false)
  }
  const chars = Array.from(draft.body.trim()).length
  const short = chars < VOICE_SAMPLE_MIN_CHARS
  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (!active || add.isPending || lock.current) return
    setAttempted(true)
    if (short) return
    lock.current = true
    onBusyChange?.(true)
    setConfirmationFailed(false)
    try {
      const result = await add.add(draft.label, draft.body)
      if (!result.sample?.id) throw new Error('Unconfirmed private material')
      if (!mounted.current) return
      onBusyChange?.(false)
      change({ label: '', body: '' })
      setSaved(true)
      if (focusAfterSave && active && !window.matchMedia?.('(pointer: coarse)').matches)
        titleField.current?.focus()
      onSaved?.()
    } catch {
      if (mounted.current && !add.isError) setConfirmationFailed(true)
    } finally {
      lock.current = false
      if (mounted.current) onBusyChange?.(false)
    }
  }
  return (
    <form onSubmit={(event) => void submit(event)} className="space-y-5">
      <div>
        <FieldLabel htmlFor={`${id}-label`}>{t('paste.label')}</FieldLabel>
        <TextField
          ref={titleField}
          id={`${id}-label`}
          autoComplete="off"
          value={draft.label}
          disabled={add.isPending}
          onChange={(event) => change({ ...draft, label: event.target.value })}
          placeholder={t('paste.labelPlaceholder')}
          className="mt-2"
        />
      </div>
      <div>
        <FieldLabel htmlFor={`${id}-body`}>{t('paste.body')}</FieldLabel>
        <Textarea
          id={`${id}-body`}
          rows={6}
          autoGrow
          value={draft.body}
          disabled={add.isPending}
          onChange={(event) => change({ ...draft, body: event.target.value })}
          placeholder={t('paste.bodyPlaceholder')}
          aria-invalid={(attempted && short) || add.isError || confirmationFailed}
          className="max-h-field mt-2"
        />
        <Typography variant="meta" className="mt-2 block">
          {t('paste.count', { count: chars, min: VOICE_SAMPLE_MIN_CHARS })}
        </Typography>
        {attempted && short && (
          <FieldMessage className="mt-2">
            {t('paste.remaining', { count: VOICE_SAMPLE_MIN_CHARS - chars })}
          </FieldMessage>
        )}
        {add.isError && <FieldMessage className="mt-2">{add.errorMessage}</FieldMessage>}
        {confirmationFailed && !add.isError && (
          <FieldMessage className="mt-2">{t('paste.confirmFailed')}</FieldMessage>
        )}
        <Typography variant="meta" role="status" className="mt-2">
          {saved ? t('paste.added') : ''}
        </Typography>
      </div>
      <div className="flex flex-wrap gap-3">
        {onBack && (
          <Button variant="ghost" disabled={add.isPending} onClick={onBack}>
            {t('action.cancel', { ns: 'common' })}
          </Button>
        )}
        <Button type="submit" variant="cta" pending={add.isPending} disabled={add.isPending}>
          {t('paste.submit')}
        </Button>
      </div>
    </form>
  )
}
