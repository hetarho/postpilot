import { useEffect, useId, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useAnswerVoicePrompt, useVoicePhotoUpload, type VoicePrompt } from '@/entities/voice'
import type { ResizedJpeg } from '@/shared/lib'
import { Button, FieldLabel, FieldMessage, Textarea, Typography } from '@/shared/ui'
import { preparePhoto, putPhoto } from '../api/photo'

type PhotoState =
  | { phase: 'none' }
  | { phase: 'converting' }
  | { phase: 'ready'; photo: ResizedJpeg; preview: string }
  | { phase: 'failed' }

/** One prompt's answer form, also opened from 검증 (VOICE-43). A rewrite starts with the saved
 *  answer and photo; the quiz closes from here, while 검증 returns to its prompt picker. */
export function AnswerForm({
  ownerId,
  voiceId,
  prompt,
  initialBody = '',
  bodyValue,
  onBodyChange,
  currentPhoto,
  focusOnMount = false,
  disabled: externallyDisabled = false,
  onSubmitStart,
  onFailure,
  onEdit,
  onBack,
  backLabel,
  backDisabled = false,
  secondaryActions,
  onDone,
}: {
  ownerId: string
  voiceId: string
  prompt: VoicePrompt
  initialBody?: string
  /** Session-owned text survives Back without keeping every question form mounted. */
  bodyValue?: string
  onBodyChange?: (body: string) => void
  currentPhoto?: { url: string; width: number; height: number }
  focusOnMount?: boolean
  disabled?: boolean
  /** A session synchronously reserves this submission before any upload or RPC. */
  onSubmitStart?: () => boolean
  onFailure?: () => void
  /** Any change to the answer or its photo. */
  onEdit?: () => void
  onBack: () => void
  backLabel?: string
  backDisabled?: boolean
  secondaryActions?: ReactNode
  onDone: () => void
}) {
  const { t } = useTranslation(['voices', 'common'])
  const id = useId()
  const fileInput = useRef<HTMLInputElement>(null)
  const answerField = useRef<HTMLTextAreaElement>(null)
  const [localBody, setBody] = useState(initialBody)
  const body = bodyValue ?? localBody
  const [photo, setPhoto] = useState<PhotoState>({ phase: 'none' })
  const [uploading, setUploading] = useState(false)
  const [uploadFailed, setUploadFailed] = useState(false)
  const [confirmationFailed, setConfirmationFailed] = useState(false)
  const answer = useAnswerVoicePrompt(ownerId, voiceId)
  const uploads = useVoicePhotoUpload(voiceId)
  const submitLock = useRef(false)

  // The preview is an object URL of the converted copy; it is released with the form.
  const preview = photo.phase === 'ready' ? photo.preview : ''
  useEffect(() => () => void (preview && URL.revokeObjectURL(preview)), [preview])

  // The list row or the form that opened this one is gone, so focus would fall to the page. A
  // rewrite puts the cursor after the answer, where more sentences go.
  useEffect(() => {
    const field = answerField.current
    if (!focusOnMount || !field || window.matchMedia?.('(pointer: coarse)').matches) return
    field.focus()
    field.setSelectionRange(field.value.length, field.value.length)
  }, [focusOnMount])

  const pick = async (file: File | undefined) => {
    if (!file) return
    onEdit?.()
    setPhoto({ phase: 'converting' })
    try {
      const converted = await preparePhoto(file)
      setPhoto({ phase: 'ready', photo: converted, preview: URL.createObjectURL(converted.blob) })
    } catch {
      setPhoto({ phase: 'failed' })
    }
  }

  const pending = uploading || answer.isPending || externallyDisabled
  const keepsPhoto = prompt.photo && currentPhoto !== undefined && photo.phase === 'none'
  const needsPhoto = prompt.photo && photo.phase !== 'ready' && !keepsPhoto
  const rewrite = initialBody !== ''
  const disabled = body.trim() === '' || needsPhoto || pending

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (disabled || submitLock.current || onSubmitStart?.() === false) return
    submitLock.current = true
    setUploadFailed(false)
    setConfirmationFailed(false)
    try {
      let uploaded: { uploadId: string; width: number; height: number } | undefined
      if (prompt.photo && photo.phase === 'ready') {
        setUploading(true)
        try {
          const handshake = await uploads.create(prompt.key)
          await putPhoto(handshake.putUrl, handshake.contentType, photo.photo.blob)
          uploaded = {
            uploadId: handshake.uploadId,
            width: photo.photo.width,
            height: photo.photo.height,
          }
        } catch {
          setUploadFailed(true)
          onFailure?.()
          return
        } finally {
          setUploading(false)
        }
      }
      const response = await answer.answer({ promptKey: prompt.key, body, photo: uploaded })
      if (!response.sample?.id || response.sample.promptKey !== prompt.key) {
        setConfirmationFailed(true)
        onFailure?.()
        return
      }
      onDone()
    } catch {
      onFailure?.()
      // The mutation's message renders under the answer.
    } finally {
      submitLock.current = false
    }
  }

  return (
    <form onSubmit={(event) => void submit(event)} className="mt-4 space-y-4">
      <Typography variant="body" as="p" className="break-words">
        {prompt.text}
      </Typography>
      {prompt.scene && (
        <Typography variant="body" className="text-content-secondary break-words">
          {prompt.scene}
        </Typography>
      )}
      {prompt.hint && (
        <Typography variant="body" className="text-content-secondary break-words">
          {prompt.hint}
        </Typography>
      )}
      {prompt.photo && (
        <div>
          <input
            ref={fileInput}
            type="file"
            accept="image/*,.heic,.heif"
            className="sr-only"
            tabIndex={-1}
            aria-label={t('prompts.pickPhoto', { ns: 'voices' })}
            onChange={(event) => void pick(event.target.files?.[0])}
          />
          {photo.phase === 'ready' && (
            <img
              src={photo.preview}
              width={photo.photo.width}
              height={photo.photo.height}
              alt={t('prompts.photoAlt', { ns: 'voices' })}
              className="max-h-field h-auto w-full rounded-md object-contain"
            />
          )}
          {keepsPhoto && (
            <img
              src={currentPhoto.url}
              width={currentPhoto.width}
              height={currentPhoto.height}
              alt={t('prompts.photoAlt', { ns: 'voices' })}
              className="max-h-field h-auto w-full rounded-md object-contain"
            />
          )}
          <Button
            variant="secondary"
            className="mt-2"
            pending={photo.phase === 'converting'}
            disabled={pending}
            onClick={() => fileInput.current?.click()}
          >
            {photo.phase === 'ready' || keepsPhoto
              ? t('prompts.changePhoto', { ns: 'voices' })
              : t('prompts.pickPhoto', { ns: 'voices' })}
          </Button>
          {photo.phase === 'converting' && (
            <span role="status" className="sr-only">
              {t('prompts.converting', { ns: 'voices' })}
            </span>
          )}
          {photo.phase === 'failed' && (
            <FieldMessage className="mt-2">
              {t('prompts.convertFailed', { ns: 'voices' })}
            </FieldMessage>
          )}
        </div>
      )}
      <div>
        <FieldLabel htmlFor={`${id}-answer`}>{t('prompts.answer', { ns: 'voices' })}</FieldLabel>
        <Textarea
          ref={answerField}
          id={`${id}-answer`}
          value={body}
          disabled={pending}
          onChange={(event) => {
            onEdit?.()
            setBody(event.target.value)
            onBodyChange?.(event.target.value)
          }}
          rows={4}
          autoGrow
          aria-invalid={answer.isError || undefined}
          className="max-h-field mt-1"
        />
        {uploadFailed && (
          <FieldMessage className="mt-2">
            {t('prompts.uploadFailed', { ns: 'voices' })}
          </FieldMessage>
        )}
        {answer.isError && <FieldMessage className="mt-2">{answer.errorMessage}</FieldMessage>}
        {confirmationFailed && (
          <FieldMessage className="mt-2">
            {t('prompts.confirmFailed', { ns: 'voices' })}
          </FieldMessage>
        )}
      </div>
      <div className="flex flex-wrap justify-end gap-2">
        <Button variant="ghost" disabled={pending || backDisabled} onClick={onBack}>
          {backLabel ?? t('prompts.back', { ns: 'voices' })}
        </Button>
        {secondaryActions}
        <Button
          type="submit"
          variant="cta"
          className="w-full sm:w-auto"
          disabled={disabled}
          pending={pending}
        >
          {rewrite ? t('prompts.rewrite', { ns: 'voices' }) : t('prompts.submit', { ns: 'voices' })}
        </Button>
      </div>
    </form>
  )
}
