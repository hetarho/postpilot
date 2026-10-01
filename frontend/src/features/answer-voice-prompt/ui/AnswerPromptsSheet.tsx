import { useEffect, useId, useRef, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import {
  useAnswerVoicePrompt,
  useVoicePhotoUpload,
  useVoicePrompts,
  useVoiceSample,
  VoiceReadinessMeter,
  type VoicePrompt,
  type VoicePromptPart,
  type VoiceReadiness,
  type VoiceSample,
} from '@/entities/voice'
import type { ResizedJpeg } from '@/shared/lib'
import {
  Badge,
  Button,
  FieldLabel,
  FieldMessage,
  Notice,
  Sheet,
  Textarea,
  Typography,
} from '@/shared/ui'
import { preparePhoto, putPhoto } from '../api/photo'

const PARTS: readonly VoicePromptPart[] = ['opening', 'description', 'closing']

/** `문항 풀기` and its sheet (VOICE-60, VOICE-64): the shared prompts grouped 글머리 · 본문 · 마무리,
 *  answered ones marked — a photo prompt on a photo the owner picks from their device. The sheet
 *  stays open after each answer and moves on to the next unanswered prompt until the owner closes
 *  it (VOICE-65). An answered prompt opens on its answer to rewrite it, so a voice whose answers
 *  fall short of 100% can still get there. For a voice not yet made, `readiness` is its meter.
 *  Answering enqueues nothing. */
export function AnswerPromptsSheet({
  ownerId,
  voiceId,
  samples,
  readiness,
  disabled = false,
}: {
  ownerId: string
  voiceId: string
  samples: readonly VoiceSample[]
  readiness?: VoiceReadiness
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
          readiness={readiness}
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
  readiness,
  onClose,
}: {
  ownerId: string
  voiceId: string
  samples: readonly VoiceSample[]
  readiness?: VoiceReadiness
  onClose: () => void
}) {
  const { t } = useTranslation('voices')
  const titleId = useId()
  const { prompts, isPending, isError } = useVoicePrompts()
  const [chosen, setChosen] = useState<VoicePrompt | null>(null)
  // Keys saved in this sitting: the save resolves before the profile refetch that lists it, and
  // without these the prompt just answered would be offered again as the next one.
  const [savedKeys, setSavedKeys] = useState<ReadonlySet<string>>(() => new Set())
  const [saved, setSaved] = useState(false)
  const answers = new Map(
    samples
      .filter((sample) => sample.kind === 'answer')
      .map((sample) => [sample.promptKey, sample] as const),
  )
  const answered = new Set([...answers.keys(), ...savedKeys])
  const allAnswered = prompts.length > 0 && prompts.every((prompt) => answered.has(prompt.key))

  /** The first unanswered prompt after `key` in the served order, wrapping, never `key` itself. */
  const nextAfter = (key: string, done: ReadonlySet<string>) => {
    const at = prompts.findIndex((prompt) => prompt.key === key)
    for (let step = 1; step < prompts.length; step++) {
      const prompt = prompts[(at + step) % prompts.length]
      if (!done.has(prompt.key)) return prompt
    }
    return null
  }

  const open = (prompt: VoicePrompt | null) => {
    setSaved(false)
    setChosen(prompt)
  }

  const answeredOne = (key: string) => {
    setSavedKeys((keys) => new Set([...keys, key]))
    setChosen(nextAfter(key, new Set([...answered, key])))
    setSaved(true)
  }

  const next = chosen ? nextAfter(chosen.key, answered) : null
  const rewriting = chosen ? answers.get(chosen.key) : undefined

  return (
    <Sheet open labelledBy={titleId} onClose={onClose}>
      <Typography variant="title" as="h2" id={titleId}>
        {t('prompts.title')}
      </Typography>
      {/* Present at rest, so the save it later names is announced (VOICE-65). */}
      <Typography variant="meta" as="p" role="status" className="mt-1">
        {saved ? t('prompts.saved') : ''}
      </Typography>
      {readiness && <VoiceReadinessMeter readiness={readiness} className="mt-4" />}
      {chosen && rewriting ? (
        <RewriteForm
          key={rewriting.id}
          ownerId={ownerId}
          voiceId={voiceId}
          prompt={chosen}
          sample={rewriting}
          onEdit={() => setSaved(false)}
          onBack={() => open(null)}
          onDone={() => answeredOne(chosen.key)}
        />
      ) : chosen ? (
        <AnswerForm
          key={chosen.key}
          ownerId={ownerId}
          voiceId={voiceId}
          prompt={chosen}
          focusOnMount
          onEdit={() => setSaved(false)}
          onBack={() => open(null)}
          onSkip={next ? () => open(next) : undefined}
          onDone={() => answeredOne(chosen.key)}
        />
      ) : isError ? (
        <FieldMessage className="mt-4">{t('prompts.loadFailed')}</FieldMessage>
      ) : isPending ? null : (
        <>
          {allAnswered && readiness && readiness.percent < 100 && (
            <Notice tone="info" role="status" className="mt-4">
              {t('prompts.short')}
            </Notice>
          )}
          {PARTS.map((part) => (
            <section key={part} className="mt-6" aria-labelledby={`${titleId}-${part}`}>
              <Typography variant="fieldTitle" as="h3" id={`${titleId}-${part}`}>
                {t(`prompts.group.${part}`)}
              </Typography>
              <ul className="divide-divider mt-2 divide-y">
                {prompts
                  .filter((prompt) => prompt.part === part)
                  .map((prompt) => {
                    const done = answered.has(prompt.key)
                    return (
                      <li key={prompt.key}>
                        <button
                          type="button"
                          // Saved this sitting but not yet listed: there is no answer to open yet.
                          disabled={done && !answers.has(prompt.key)}
                          onClick={() => open(prompt)}
                          className="hover:bg-row-bg-hover active:bg-row-bg-active disabled:text-content-tertiary flex min-h-11 w-full items-center gap-2 py-3 text-left"
                        >
                          <Typography variant="body" as="span" className="min-w-0 flex-1">
                            {prompt.text}
                          </Typography>
                          {prompt.photo && <Badge>{t('prompts.photo')}</Badge>}
                          {done && <Badge tone="accent">{t('prompts.answered')}</Badge>}
                        </button>
                      </li>
                    )
                  })}
              </ul>
            </section>
          ))}
        </>
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
  onDone,
}: {
  ownerId: string
  voiceId: string
  prompt: VoicePrompt
  sample: VoiceSample
  onEdit: () => void
  onBack: () => void
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
      onDone={onDone}
    />
  )
}

type PhotoState =
  | { phase: 'none' }
  | { phase: 'converting' }
  | { phase: 'ready'; photo: ResizedJpeg; preview: string }
  | { phase: 'failed' }

/** One prompt's answer form, also opened from 검증 for a prompt not yet answered (VOICE-43).
 *  `onSkip` adds 건너뛰기, and `focusOnMount` puts the cursor in the answer — the sheet's run of
 *  prompts passes both, 검증's single answer neither (VOICE-65). A rewrite passes the answer it
 *  replaces as `initialBody` and its photo as `currentPhoto`, kept unless another is picked. */
export function AnswerForm({
  ownerId,
  voiceId,
  prompt,
  initialBody = '',
  currentPhoto,
  focusOnMount = false,
  onEdit,
  onBack,
  onSkip,
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
  onSkip?: () => void
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
          {t('prompts.back', { ns: 'voices' })}
        </Button>
        {onSkip && (
          <Button variant="ghost" disabled={pending} onClick={onSkip}>
            {t('prompts.skip', { ns: 'voices' })}
          </Button>
        )}
        <Button type="submit" variant="cta" disabled={disabled} pending={pending}>
          {rewrite ? t('prompts.rewrite', { ns: 'voices' }) : t('prompts.submit', { ns: 'voices' })}
        </Button>
      </div>
    </form>
  )
}
