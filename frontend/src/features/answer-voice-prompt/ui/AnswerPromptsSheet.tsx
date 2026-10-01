import { useEffect, useId, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import {
  useAnswerVoicePrompt,
  useVoicePhotoUpload,
  useVoicePrompts,
  useVoiceSample,
  VoiceReadinessMeter,
  type VoicePrompt,
  type VoiceProfile,
  type VoiceSample,
} from '@/entities/voice'
import type { ResizedJpeg } from '@/shared/lib'
import { Button, FieldLabel, FieldMessage, Notice, Sheet, Textarea, Typography } from '@/shared/ui'
import { preparePhoto, putPhoto } from '../api/photo'

/** One prompt at a time (VOICE-65). Once all are answered, an unmade voice below 100% cycles
 *  through existing answers so the owner can add sentences without losing the chosen photo. */
export function AnswerPromptsSheet({
  ownerId,
  voiceId,
  samples,
  profile,
  renderMakeVoice,
  disabled = false,
}: {
  ownerId: string
  voiceId: string
  samples: readonly VoiceSample[]
  profile: VoiceProfile
  renderMakeVoice: (onStarted: () => void) => ReactNode
  disabled?: boolean
}) {
  const { t } = useTranslation('voices')
  const [open, setOpen] = useState(false)
  return (
    <>
      <Button variant="secondary" disabled={disabled} onClick={() => setOpen(true)}>
        {t('prompts.open')}
      </Button>
      {open && (
        <PromptsPanel
          ownerId={ownerId}
          voiceId={voiceId}
          samples={samples}
          profile={profile}
          renderMakeVoice={renderMakeVoice}
          onClose={() => setOpen(false)}
        />
      )}
    </>
  )
}

function PromptsPanel({
  ownerId,
  voiceId,
  samples,
  profile,
  renderMakeVoice,
  onClose,
}: {
  ownerId: string
  voiceId: string
  samples: readonly VoiceSample[]
  profile: VoiceProfile
  renderMakeVoice: (onStarted: () => void) => ReactNode
  onClose: () => void
}) {
  const { t } = useTranslation('voices')
  const titleId = useId()
  const { prompts, isPending, isError } = useVoicePrompts()
  // A save resolves before the profile refetch lists it. These keys advance the question
  // immediately while the server remains the source of the readiness percentage.
  const [savedKeys, setSavedKeys] = useState<ReadonlySet<string>>(() => new Set())
  const [saved, setSaved] = useState(false)
  const [rewriteIndex, setRewriteIndex] = useState(0)
  const answers = new Map(
    samples
      .filter((sample) => sample.kind === 'answer')
      .map((sample) => [sample.promptKey, sample] as const),
  )
  const answered = new Set([...answers.keys(), ...savedKeys])
  const unanswered = prompts.find((prompt) => !answered.has(prompt.key))
  const allAnswered = prompts.length > 0 && !unanswered
  const needsMore = !profile.made && profile.readiness.percent < 100
  const rewriteCandidates =
    allAnswered && needsMore ? prompts.filter((prompt) => answers.has(prompt.key)) : []
  const chosen = unanswered ?? rewriteCandidates[rewriteIndex % rewriteCandidates.length]
  const rewriting = chosen && allAnswered ? answers.get(chosen.key) : undefined

  const answeredOne = (key: string, wasRewrite = false) => {
    setSavedKeys((keys) => new Set([...keys, key]))
    if (wasRewrite) setRewriteIndex((index) => index + 1)
    setSaved(true)
  }

  return (
    <Sheet open labelledBy={titleId} onClose={onClose}>
      <Typography variant="title" as="h2" id={titleId}>
        {t('prompts.title')}
      </Typography>
      {/* Present at rest, so the save it later names is announced (VOICE-65). */}
      <Typography variant="meta" as="p" role="status" className="mt-1">
        {saved ? t('prompts.saved') : ''}
      </Typography>
      {!profile.made && (
        <div className="mt-4">
          <VoiceReadinessMeter readiness={profile.readiness} />
          {profile.readiness.percent >= 80 && profile.readiness.percent < 100 && (
            <Typography variant="body" as="p" className="mt-2">
              {t('prompts.almostReady')}
            </Typography>
          )}
          {profile.readiness.percent >= 100 && (
            <div className="mt-4">{renderMakeVoice(onClose)}</div>
          )}
        </div>
      )}
      {allAnswered && needsMore && (
        <Notice tone="info" className="mt-4">
          {t('prompts.short')}
        </Notice>
      )}
      {isError ? (
        <FieldMessage className="mt-4">{t('prompts.loadFailed')}</FieldMessage>
      ) : isPending ? null : chosen && rewriting ? (
        <RewriteForm
          key={rewriting.id}
          ownerId={ownerId}
          voiceId={voiceId}
          prompt={chosen}
          sample={rewriting}
          onEdit={() => setSaved(false)}
          onBack={onClose}
          backLabel={t('prompts.close')}
          onDone={() => answeredOne(chosen.key, true)}
        />
      ) : chosen ? (
        <AnswerForm
          key={chosen.key}
          ownerId={ownerId}
          voiceId={voiceId}
          prompt={chosen}
          focusOnMount
          onEdit={() => setSaved(false)}
          onBack={onClose}
          backLabel={t('prompts.close')}
          onDone={() => answeredOne(chosen.key)}
        />
      ) : (
        <div className="mt-6">
          <Typography variant="body" as="p">
            {t('prompts.complete')}
          </Typography>
          <Button variant="secondary" className="mt-4" onClick={onClose}>
            {t('prompts.close')}
          </Button>
        </div>
      )}
    </Sheet>
  )
}

/** An answered prompt reopened on its answer: saving replaces it, on the same photo unless the
 *  owner picks another (VOICE-60). */
function RewriteForm({
  ownerId,
  voiceId,
  prompt,
  sample,
  onEdit,
  onBack,
  backLabel,
  onDone,
}: {
  ownerId: string
  voiceId: string
  prompt: VoicePrompt
  sample: VoiceSample
  onEdit: () => void
  onBack: () => void
  backLabel: string
  onDone: () => void
}) {
  const { t } = useTranslation('voices')
  const { detail, isError } = useVoiceSample(ownerId, voiceId, sample.id)
  if (isError) return <FieldMessage className="mt-4">{t('prompts.answerLoadFailed')}</FieldMessage>
  if (!detail) return null
  return (
    <AnswerForm
      ownerId={ownerId}
      voiceId={voiceId}
      prompt={prompt}
      initialBody={detail.body}
      currentPhoto={
        detail.photoUrl
          ? { url: detail.photoUrl, width: detail.photoWidth, height: detail.photoHeight }
          : undefined
      }
      focusOnMount
      onEdit={onEdit}
      onBack={onBack}
      backLabel={backLabel}
      onDone={onDone}
    />
  )
}

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
  currentPhoto,
  focusOnMount = false,
  onEdit,
  onBack,
  backLabel,
  onDone,
}: {
  ownerId: string
  voiceId: string
  prompt: VoicePrompt
  initialBody?: string
  currentPhoto?: { url: string; width: number; height: number }
  focusOnMount?: boolean
  /** Any change to the answer or its photo. */
  onEdit?: () => void
  onBack: () => void
  backLabel?: string
  onDone: () => void
}) {
  const { t } = useTranslation(['voices', 'common'])
  const id = useId()
  const fileInput = useRef<HTMLInputElement>(null)
  const answerField = useRef<HTMLTextAreaElement>(null)
  const [body, setBody] = useState(initialBody)
  const [photo, setPhoto] = useState<PhotoState>({ phase: 'none' })
  const [uploading, setUploading] = useState(false)
  const [uploadFailed, setUploadFailed] = useState(false)
  const answer = useAnswerVoicePrompt(ownerId, voiceId)
  const uploads = useVoicePhotoUpload(voiceId)

  // The preview is an object URL of the converted copy; it is released with the form.
  const preview = photo.phase === 'ready' ? photo.preview : ''
  useEffect(() => () => void (preview && URL.revokeObjectURL(preview)), [preview])

  // The list row or the form that opened this one is gone, so focus would fall to the page. A
  // rewrite puts the cursor after the answer, where more sentences go.
  useEffect(() => {
    const field = answerField.current
    if (!focusOnMount || !field) return
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

  const pending = uploading || answer.isPending
  const keepsPhoto = prompt.photo && currentPhoto !== undefined && photo.phase === 'none'
  const needsPhoto = prompt.photo && photo.phase !== 'ready' && !keepsPhoto
  const rewrite = initialBody !== ''
  const disabled = body.trim() === '' || needsPhoto || pending

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (disabled) return
    setUploadFailed(false)
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
          return
        } finally {
          setUploading(false)
        }
      }
      await answer.answer({ promptKey: prompt.key, body, photo: uploaded })
      onDone()
    } catch {
      // The mutation's message renders under the answer.
    }
  }

  return (
    <form onSubmit={(event) => void submit(event)} className="mt-4 space-y-4">
      <Typography variant="body" as="p" className="break-words">
        {prompt.text}
      </Typography>
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
          onChange={(event) => {
            onEdit?.()
            setBody(event.target.value)
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
      </div>
      <div className="flex flex-wrap justify-end gap-2">
        <Button variant="ghost" disabled={pending} onClick={onBack}>
          {backLabel ?? t('prompts.back', { ns: 'voices' })}
        </Button>
        <Button type="submit" variant="cta" disabled={disabled} pending={pending}>
          {rewrite ? t('prompts.rewrite', { ns: 'voices' }) : t('prompts.submit', { ns: 'voices' })}
        </Button>
      </div>
    </form>
  )
}
