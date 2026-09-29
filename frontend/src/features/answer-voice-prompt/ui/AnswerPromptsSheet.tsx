import { useEffect, useId, useRef, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import {
  useAnswerVoicePrompt,
  useVoicePhotoUpload,
  useVoicePrompts,
  type VoicePrompt,
  type VoicePromptPart,
  type VoiceSample,
} from '@/entities/voice'
import type { ResizedJpeg } from '@/shared/lib'
import { Badge, Button, FieldLabel, FieldMessage, Sheet, Textarea, Typography } from '@/shared/ui'
import { preparePhoto, putPhoto } from '../api/photo'

const PARTS: readonly VoicePromptPart[] = ['opening', 'description', 'closing']

/** `문항 풀기` and its sheet (VOICE-60, VOICE-64): the shared prompts grouped 글머리 · 본문 · 마무리,
 *  answered ones marked, and one prompt answered at a time — a photo prompt on a photo the owner
 *  picks from their device. Answering enqueues nothing. */
export function AnswerPromptsSheet({
  ownerId,
  voiceId,
  samples,
  disabled = false,
}: {
  ownerId: string
  voiceId: string
  samples: readonly VoiceSample[]
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
  onClose,
}: {
  ownerId: string
  voiceId: string
  samples: readonly VoiceSample[]
  onClose: () => void
}) {
  const { t } = useTranslation('voices')
  const titleId = useId()
  const { prompts, isPending, isError } = useVoicePrompts()
  const [chosen, setChosen] = useState<VoicePrompt | null>(null)
  const answered = new Set(
    samples.filter((sample) => sample.kind === 'answer').map((sample) => sample.promptKey),
  )

  return (
    <Sheet open labelledBy={titleId} onClose={onClose}>
      <Typography variant="title" as="h2" id={titleId}>
        {t('prompts.title')}
      </Typography>
      {chosen ? (
        <AnswerForm
          ownerId={ownerId}
          voiceId={voiceId}
          prompt={chosen}
          onBack={() => setChosen(null)}
          onDone={onClose}
        />
      ) : isError ? (
        <FieldMessage className="mt-4">{t('prompts.loadFailed')}</FieldMessage>
      ) : isPending ? null : (
        PARTS.map((part) => (
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
                        disabled={done}
                        onClick={() => setChosen(prompt)}
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
        ))
      )}
    </Sheet>
  )
}

type PhotoState =
  | { phase: 'none' }
  | { phase: 'converting' }
  | { phase: 'ready'; photo: ResizedJpeg; preview: string }
  | { phase: 'failed' }

/** One prompt's answer form, also opened from 검증 for a prompt not yet answered (VOICE-43). */
export function AnswerForm({
  ownerId,
  voiceId,
  prompt,
  onBack,
  onDone,
}: {
  ownerId: string
  voiceId: string
  prompt: VoicePrompt
  onBack: () => void
  onDone: () => void
}) {
  const { t } = useTranslation(['voices', 'common'])
  const id = useId()
  const fileInput = useRef<HTMLInputElement>(null)
  const [body, setBody] = useState('')
  const [photo, setPhoto] = useState<PhotoState>({ phase: 'none' })
  const [uploading, setUploading] = useState(false)
  const [uploadFailed, setUploadFailed] = useState(false)
  const answer = useAnswerVoicePrompt(ownerId, voiceId)
  const uploads = useVoicePhotoUpload(voiceId)

  // The preview is an object URL of the converted copy; it is released with the form.
  const preview = photo.phase === 'ready' ? photo.preview : ''
  useEffect(() => () => void (preview && URL.revokeObjectURL(preview)), [preview])

  const pick = async (file: File | undefined) => {
    if (!file) return
    setPhoto({ phase: 'converting' })
    try {
      const converted = await preparePhoto(file)
      setPhoto({ phase: 'ready', photo: converted, preview: URL.createObjectURL(converted.blob) })
    } catch {
      setPhoto({ phase: 'failed' })
    }
  }

  const pending = uploading || answer.isPending
  const needsPhoto = prompt.photo && photo.phase !== 'ready'
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
          <Button
            variant="secondary"
            className="mt-2"
            pending={photo.phase === 'converting'}
            disabled={pending}
            onClick={() => fileInput.current?.click()}
          >
            {photo.phase === 'ready'
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
          id={`${id}-answer`}
          value={body}
          onChange={(event) => setBody(event.target.value)}
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
        <Button type="submit" variant="cta" disabled={disabled} pending={pending}>
          {t('prompts.submit', { ns: 'voices' })}
        </Button>
      </div>
    </form>
  )
}
