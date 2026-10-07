import { useCallback, useEffect, useId, useMemo, useRef, useState } from 'react'
import { useActorRef, useSelector } from '@xstate/react'
import { useTranslation } from 'react-i18next'
import {
  useVoiceMaterialEditor,
  useVoicePhotoUpload,
  useVoiceSample,
  type VoicePrompt,
  type VoiceSampleDetail,
} from '@/entities/voice'
import {
  AppFailureMessage,
  Button,
  FieldLabel,
  FieldMessage,
  Notice,
  Textarea,
  TextField,
  Typography,
} from '@/shared/ui'
import {
  materialDraftChanged,
  materialDraftProblem,
  materialEditMachine,
  type MaterialPhotoChoice,
  type MaterialEditDraft,
} from '../model/material-edit-machine'
import { VOICE_MATERIAL_EDIT_MIN_POST_CHARS } from '../config'
import { prepareMaterialPhoto, putMaterialPhoto } from '../api/photo'

export interface EditVoiceMaterialProps {
  ownerId: string
  voiceId: string
  detail: VoiceSampleDetail
  prompt?: VoicePrompt
  blocked?: boolean
  onSaved: () => void
  onCancel: () => void
  onBusyChange?: (busy: boolean) => void
}
export function EditVoiceMaterial(props: EditVoiceMaterialProps) {
  return (
    <ScopedEditor
      key={JSON.stringify([props.ownerId, props.voiceId, props.detail.sample.id])}
      {...props}
    />
  )
}
function ScopedEditor({
  ownerId,
  voiceId,
  detail,
  prompt,
  blocked = false,
  onSaved,
  onCancel,
  onBusyChange,
}: EditVoiceMaterialProps) {
  const { t } = useTranslation('voiceMaterialEdit')
  const id = useId()
  const scopeKey =
    ownerId && voiceId && detail.sample.id
      ? JSON.stringify([ownerId, voiceId, detail.sample.id])
      : ''
  const editor = useVoiceMaterialEditor(ownerId, voiceId)
  const uploads = useVoicePhotoUpload(voiceId)
  const current = useVoiceSample(ownerId, voiceId, detail.sample.id)
  const latestReader = useRef(current.refresh)
  useEffect(() => {
    latestReader.current = current.refresh
  }, [current.refresh])
  const services = useMemo(
    () => ({
      update: editor.update,
      read: () => latestReader.current(),
      upload: async (photo: { blob: Blob; width: number; height: number }, signal: AbortSignal) => {
        const receipt = await uploads.create(detail.sample.promptKey)
        if (signal.aborted) throw new Error('Obsolete material photo')
        await putMaterialPhoto(receipt.putUrl, receipt.contentType, photo.blob, signal)
        return { uploadId: receipt.uploadId, width: photo.width, height: photo.height }
      },
    }),
    [editor, uploads, detail.sample.promptKey],
  )
  const actor = useActorRef(materialEditMachine, {
    input: {
      scopeKey,
      baseline: detail,
      photoRequired: prompt?.photo ?? (detail.sample.kind === 'answer' && detail.sample.hasPhoto),
      blocked,
      minimumPostChars: VOICE_MATERIAL_EDIT_MIN_POST_CHARS,
      services,
      requestKey: () => crypto.randomUUID(),
    },
  })
  const snapshot = useSelector(actor, (value) => value)
  const context = snapshot.context
  const busy = snapshot.hasTag('busy')
  const [converting, setConverting] = useState(false)
  const [photoFailed, setPhotoFailed] = useState(false)
  const file = useRef<HTMLInputElement>(null)
  const lifetime = useRef({ mounted: true, sequence: 0 })
  useEffect(() => {
    const active = lifetime.current
    active.mounted = true
    return () => {
      active.mounted = false
      active.sequence++
    }
  }, [])
  const busyCallback = useRef(onBusyChange)
  useEffect(() => {
    busyCallback.current = onBusyChange
  }, [onBusyChange])
  useEffect(() => {
    busyCallback.current?.(busy || converting)
  }, [busy, converting])
  useEffect(() => () => busyCallback.current?.(false), [])
  const notified = useRef(false)
  useEffect(() => {
    if (notified.current) return
    if (snapshot.matches('saved')) {
      notified.current = true
      onSaved()
    }
    if (snapshot.matches('cancelled')) {
      notified.current = true
      onCancel()
    }
  }, [snapshot, onSaved, onCancel])
  const photo = context.draft.photo
  const preview = photo.mode === 'replace' ? photo.preview : ''
  useEffect(
    () => () => {
      if (preview) URL.revokeObjectURL(preview)
    },
    [preview],
  )
  const change = useCallback(
    (patch: Partial<MaterialEditDraft>) => actor.send({ type: 'CHANGE', scopeKey, patch }),
    [actor, scopeKey],
  )
  const pick = async (selected?: File) => {
    if (!selected || busy || blocked) return
    const sequence = ++lifetime.current.sequence
    setConverting(true)
    setPhotoFailed(false)
    try {
      const converted = await prepareMaterialPhoto(selected)
      if (!lifetime.current.mounted || lifetime.current.sequence !== sequence) return
      change({
        photo: { mode: 'replace', photo: converted, preview: URL.createObjectURL(converted.blob) },
      })
    } catch {
      if (lifetime.current.mounted && lifetime.current.sequence === sequence) setPhotoFailed(true)
    } finally {
      if (lifetime.current.mounted && lifetime.current.sequence === sequence) setConverting(false)
    }
  }
  const problem = materialDraftProblem(context)
  const problemKey = blocked
    ? 'blocked'
    : problem === 'body'
      ? detail.sample.kind === 'post'
        ? 'bodyShort'
        : 'bodyRequired'
      : problem === 'photo'
        ? 'photoRequired'
        : problem
  const showPhoto = context.draft.photo.mode === 'keep' ? context.baseline.photoUrl : preview
  const dimensions =
    photo.mode === 'replace'
      ? photo.photo
      : { width: context.baseline.photoWidth, height: context.baseline.photoHeight }
  const send = (type: 'SAVE' | 'CANCEL' | 'RELOAD' | 'RETRY') => actor.send({ type, scopeKey })
  return (
    <div className="space-y-4">
      <Typography variant="body" as="p" className="text-content-secondary">
        {t('saveHelp')}
      </Typography>
      {detail.sample.kind === 'post' && (
        <div>
          <FieldLabel htmlFor={`${id}-label`}>{t('label')}</FieldLabel>
          <TextField
            id={`${id}-label`}
            value={context.draft.label}
            disabled={busy || blocked}
            onChange={(event) => change({ label: event.target.value })}
            className="mt-2"
          />
        </div>
      )}
      {detail.sample.kind === 'answer' && prompt && (
        <Typography variant="label" as="p" className="break-words">
          {prompt.text}
        </Typography>
      )}
      <div>
        <FieldLabel htmlFor={`${id}-body`}>
          {t(detail.sample.kind === 'post' ? 'body' : 'answer')}
        </FieldLabel>
        <Textarea
          id={`${id}-body`}
          value={context.draft.body}
          rows={6}
          autoGrow
          disabled={busy || blocked}
          onChange={(event) => change({ body: event.target.value })}
          className="max-h-field mt-2"
        />
        <Typography variant="meta" as="p" className="mt-2">
          {t('count', { count: Array.from(context.draft.body.trim()).length })}
        </Typography>
      </div>
      {(detail.sample.hasPhoto || prompt?.photo) && (
        <div>
          {showPhoto ? (
            <img
              src={showPhoto}
              width={dimensions.width}
              height={dimensions.height}
              alt={t('photo')}
              className="max-h-field h-auto w-full rounded-md object-contain"
            />
          ) : photo.mode === 'keep' && detail.sample.hasPhoto ? (
            <Typography variant="label" as="p">
              {t('photoUnavailable')}
            </Typography>
          ) : null}
          <input
            ref={file}
            type="file"
            accept="image/*,.heic,.heif"
            aria-label={t('replacePhoto')}
            className="sr-only"
            tabIndex={-1}
            onChange={(event) => {
              const selected = event.target.files?.[0]
              event.target.value = ''
              void pick(selected)
            }}
          />
          <div className="mt-2 flex flex-wrap gap-2">
            {prompt?.photo && (
              <Button
                variant="secondary"
                disabled={busy || converting || blocked}
                onClick={() => file.current?.click()}
              >
                {t('replacePhoto')}
              </Button>
            )}
            {photo.mode !== 'keep' && (
              <Button
                variant="ghost"
                disabled={busy || blocked}
                onClick={() => change({ photo: { mode: 'keep' } as MaterialPhotoChoice })}
              >
                {t('keepPhoto')}
              </Button>
            )}
            {detail.sample.hasPhoto && prompt !== undefined && !prompt.photo && (
              <Button
                variant="danger"
                disabled={busy || converting || blocked}
                onClick={() => change({ photo: { mode: 'remove' } })}
              >
                {t('removePhoto')}
              </Button>
            )}
          </div>
          {converting && (
            <Typography variant="label" role="status">
              {t('preparingPhoto')}
            </Typography>
          )}
          {photoFailed && <FieldMessage>{t('photoFailed')}</FieldMessage>}
        </div>
      )}
      {context.latest && (
        <div className="bg-surface-recessed rounded-md p-4">
          <Typography variant="label" as="p">
            {t('latest')}
          </Typography>
          <Typography variant="body" as="p" className="mt-2 break-words whitespace-pre-wrap">
            {context.latest.body}
          </Typography>
        </div>
      )}
      {snapshot.matches('conflict') && (
        <Notice tone="warning" role="status">
          {t('conflict')}
          <Button variant="secondary" onClick={() => send('RELOAD')}>
            {t('reload')}
          </Button>
        </Notice>
      )}
      {snapshot.matches('saveFailed') && (
        <Notice tone="warning" role="status">
          {t('unknownSave')}
          <Button variant="secondary" onClick={() => send('RETRY')}>
            {t('retry')}
          </Button>
          <Button variant="ghost" onClick={() => send('RELOAD')}>
            {t('reload')}
          </Button>
        </Notice>
      )}
      {snapshot.matches('uploadFailed') && (
        <Button variant="secondary" onClick={() => send('RETRY')}>
          {t('retry')}
        </Button>
      )}
      {context.failure && (
        <FieldMessage>
          <AppFailureMessage failure={context.failure} />
        </FieldMessage>
      )}
      {problemKey && (
        <Typography variant="label" as="p" role="status">
          {t(problemKey)}
        </Typography>
      )}
      <div className="flex flex-wrap justify-end gap-3">
        <Button variant="ghost" disabled={busy} onClick={() => send('CANCEL')}>
          {t('cancel')}
        </Button>
        <Button
          variant="cta"
          disabled={
            busy ||
            converting ||
            blocked ||
            !!problem ||
            !materialDraftChanged(context) ||
            !snapshot.matches('editing')
          }
          pending={busy}
          onClick={() => send('SAVE')}
        >
          {t('save')}
        </Button>
      </div>
    </div>
  )
}
