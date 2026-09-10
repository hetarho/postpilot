import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Link, useBlocker, useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import {
  CLIP_PROJECT_LIMITS,
  CLIP_RATIOS,
  emptyClipProject,
  normalizeClipProject,
  projectDraft,
  useClipProjectMutations,
  validClipProject,
  type ClipProject,
  type ClipProjectDraft,
} from '@/entities/clip-project'
import { useClipTemplates } from '@/entities/clip-template'
import { appFailureFromConnect } from '@/shared/api'
import {
  ActionBar,
  AppFailureMessage,
  Button,
  Dialog,
  FieldLabel,
  FieldMessage,
  Listbox,
  Textarea,
  TextField,
  Typography,
  buttonStyles,
} from '@/shared/ui'

const INPUT = {
  autoComplete: 'off',
  autoCapitalize: 'sentences',
  autoCorrect: 'on',
  enterKeyHint: 'next',
} as const
export function ClipProjectForm({
  ownerId,
  stored,
  onUploadAllowed,
  children,
  status,
  progress,
  disabled = false,
  actions,
  refusal,
  showStatus = true,
  onSaveStateChange,
}: {
  ownerId: string
  stored?: ClipProject
  onUploadAllowed?: (allowed: boolean) => void
  children?: ReactNode
  status?: (saved: boolean) => ReactNode
  progress?: ReactNode
  disabled?: boolean
  actions?: (ready: boolean, dirty: boolean) => ReactNode
  refusal?: ReactNode
  showStatus?: boolean
  onSaveStateChange?: (saved: boolean) => void
}) {
  const { t } = useTranslation('clips')
  const navigate = useNavigate()
  const templates = useClipTemplates(ownerId)
  const [draft, setDraft] = useState<ClipProjectDraft>(() =>
    stored ? projectDraft(stored) : emptyClipProject(),
  )
  const [seconds, setSeconds] = useState(String(draft.targetDurationMs / 1000))
  const [baseline, setBaseline] = useState(JSON.stringify(normalizeClipProject(draft)))
  const [saved, setSaved] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const { save, remove } = useClipProjectMutations(ownerId)
  const submitting = useRef(false)
  const leaving = useRef(false)
  const selected = templates.templates.find((v) => v.id === draft.videoTemplateId)
  const valid = validClipProject(draft, selected?.informationFields)
  const dirty = JSON.stringify(normalizeClipProject(draft)) !== baseline
  const storedJSON = stored ? JSON.stringify(normalizeClipProject(stored)) : baseline
  const synced = storedJSON === baseline
  const pending = disabled || save.isPending || remove.isPending
  if (!synced && !dirty && !save.isPending) {
    const refreshed = JSON.parse(storedJSON) as ClipProjectDraft
    setDraft(refreshed)
    setSeconds(String(refreshed.targetDurationMs / 1000))
    setBaseline(storedJSON)
  }
  const guard = () => dirty && !leaving.current
  const blocker = useBlocker({
    shouldBlockFn: guard,
    enableBeforeUnload: guard,
    withResolver: true,
  })
  useEffect(() => {
    onUploadAllowed?.(valid && !dirty && synced && !pending)
  }, [valid, dirty, synced, pending, onUploadAllowed])
  const change = <K extends keyof ClipProjectDraft>(key: K, value: ClipProjectDraft[K]) => {
    setSaved(false)
    onSaveStateChange?.(false)
    setDraft((current) => ({ ...current, [key]: value }))
  }
  const failure = save.error ?? remove.error
  const submit = async () => {
    if (pending || !valid || (!dirty && stored) || submitting.current) return
    submitting.current = true
    try {
      const value = await save.mutateAsync({ id: stored?.id, draft })
      setDraft(projectDraft(value))
      setBaseline(JSON.stringify(normalizeClipProject(value)))
      setSaved(true)
      onSaveStateChange?.(true)
      if (!stored) {
        leaving.current = true
        await navigate({ to: '/clips/$clipId', params: { clipId: value.id }, replace: true })
      }
    } catch {
      /* Preserve all local fields after a refusal. */
    } finally {
      submitting.current = false
    }
  }
  const deleteProject = async () => {
    if (pending || !stored || submitting.current) return
    submitting.current = true
    try {
      await remove.mutateAsync(stored.id)
      leaving.current = true
      await navigate({ to: '/clips', replace: true })
    } catch {
      setConfirmDelete(false)
    } finally {
      submitting.current = false
    }
  }
  return (
    <>
      {progress}
      {showStatus && (
        <div role="status" aria-live="polite" className="mt-4">
          {status
            ? status(saved)
            : saved && <Typography variant="meta">{t('project.saved')}</Typography>}
        </div>
      )}
      <form
        id="clip-project-form"
        className="mt-6"
        onSubmit={(event) => {
          event.preventDefault()
          void submit()
        }}
      >
        <fieldset disabled={pending} className="min-w-0 space-y-6">
          <div>
            <FieldLabel htmlFor="clip-title">{t('project.name')}</FieldLabel>
            <TextField
              id="clip-title"
              type="text"
              inputMode="text"
              {...INPUT}
              value={draft.title}
              onChange={(event) => change('title', event.target.value)}
              aria-invalid={
                !draft.title.trim() ||
                Array.from(draft.title.trim()).length > CLIP_PROJECT_LIMITS.title
              }
            />
            {(!draft.title.trim() ||
              Array.from(draft.title.trim()).length > CLIP_PROJECT_LIMITS.title) && (
              <FieldMessage>{t('project.titleLimit')}</FieldMessage>
            )}
          </div>
          <div>
            <FieldLabel id="clip-template-label" htmlFor="clip-template">
              {t('project.template')}
            </FieldLabel>
            <Listbox
              id="clip-template"
              aria-labelledby="clip-template-label"
              value={draft.videoTemplateId}
              onChange={(value) => change('videoTemplateId', value)}
              options={[
                { value: '', label: t('project.chooseTemplate') },
                ...templates.templates.map((v) => ({ value: v.id, label: v.name })),
              ]}
            />
            {templates.isPending && <Typography variant="body">{t('project.loading')}</Typography>}
            {templates.isError && (
              <div role="alert">
                <AppFailureMessage failure={appFailureFromConnect(templates.error)} />
                <Button variant="ghost" onClick={() => void templates.refetch()}>
                  {t('project.retry')}
                </Button>
              </div>
            )}
            {!templates.isPending && !templates.isError && !templates.templates.length && (
              <Typography variant="body" className="mt-2">
                {t('project.noTemplates')}{' '}
                <Link to="/video-templates/new" className={buttonStyles({ variant: 'ghost' })}>
                  {t('directory.create')}
                </Link>
              </Typography>
            )}
            {!selected && !!draft.videoTemplateId && (
              <FieldMessage>{t('project.detachedTemplate')}</FieldMessage>
            )}
          </div>
          {selected?.informationFields.map((field, index) => {
            const answer = draft.answers.find((a) => a.label === field.label)?.text ?? ''
            const invalid = !answer.trim() || Array.from(answer).length > CLIP_PROJECT_LIMITS.answer
            return (
              <div key={field.label}>
                <FieldLabel htmlFor={`clip-answer-${index}`}>{field.label}</FieldLabel>
                <Typography variant="body" className="text-content-secondary mb-2 break-words">
                  {field.prompt}
                </Typography>
                <Textarea
                  id={`clip-answer-${index}`}
                  inputMode="text"
                  {...INPUT}
                  autoGrow
                  value={answer}
                  aria-invalid={invalid}
                  onChange={(event) =>
                    change('answers', [
                      ...draft.answers.filter((a) => a.label !== field.label),
                      { label: field.label, text: event.target.value },
                    ])
                  }
                />
                {invalid && <FieldMessage>{t('project.answerLimit')}</FieldMessage>}
              </div>
            )
          })}
          <div>
            {stored ? (
              <>
                <Typography variant="fieldTitle" as="p">
                  {t('project.ratio')}
                </Typography>
                <Typography variant="body" className="mt-2">
                  {t(`ratio.${stored.ratio}`)}
                </Typography>
                <Typography variant="body" className="text-content-secondary mt-2">
                  {t('project.fixedRatio')}
                </Typography>
              </>
            ) : (
              <>
                <FieldLabel id="clip-ratio-label" htmlFor="clip-ratio">
                  {t('project.ratio')}
                </FieldLabel>
                <Listbox
                  id="clip-ratio"
                  aria-labelledby="clip-ratio-label"
                  value={draft.ratio}
                  onChange={(value) => change('ratio', value)}
                  options={CLIP_RATIOS.map((value) => ({ value, label: t(`ratio.${value}`) }))}
                />
              </>
            )}
          </div>
          <div>
            <FieldLabel htmlFor="clip-duration">{t('project.duration')}</FieldLabel>
            <TextField
              id="clip-duration"
              type="number"
              inputMode="decimal"
              {...INPUT}
              min={CLIP_PROJECT_LIMITS.minSeconds}
              max={CLIP_PROJECT_LIMITS.maxSeconds}
              step="any"
              value={seconds}
              onChange={(event) => {
                setSeconds(event.target.value)
                change('targetDurationMs', Math.round(Number(event.target.value) * 1000))
              }}
              aria-invalid={
                !Number.isFinite(draft.targetDurationMs) ||
                draft.targetDurationMs < CLIP_PROJECT_LIMITS.minSeconds * 1000 ||
                draft.targetDurationMs > CLIP_PROJECT_LIMITS.maxSeconds * 1000
              }
            />
            <Typography variant="body" className="text-content-secondary mt-2">
              {t('project.durationHelp')}
            </Typography>
          </div>
        </fieldset>
      </form>
      {children}
      <ActionBar className="mt-auto">
        {refusal}
        {failure && (
          <div role="alert" className="mb-3">
            <AppFailureMessage failure={appFailureFromConnect(failure)} />
          </div>
        )}
        <div className="flex flex-wrap justify-end gap-3">
          {stored && (
            <Button variant="danger" disabled={pending} onClick={() => setConfirmDelete(true)}>
              {t('project.delete')}
            </Button>
          )}
          <Button
            form="clip-project-form"
            type="submit"
            variant={actions && !dirty ? 'secondary' : 'cta'}
            className="w-full sm:w-auto"
            pending={save.isPending}
            disabled={!valid || (!!stored && !dirty) || pending}
          >
            {t(stored ? 'project.save' : 'project.create')}
          </Button>
          {actions?.(valid && !dirty && synced && !pending, dirty)}
        </div>
      </ActionBar>
      <Dialog
        open={confirmDelete}
        title={t('project.deleteTitle')}
        confirmLabel={t('project.delete')}
        onClose={() => setConfirmDelete(false)}
        onConfirm={() => void deleteProject()}
        pending={remove.isPending}
      >
        {t('project.deleteBody')}
      </Dialog>
      <Dialog
        open={blocker.status === 'blocked'}
        title={t('project.leaveTitle')}
        confirmLabel={t('project.leave')}
        onClose={() => blocker.reset?.()}
        onConfirm={() => blocker.proceed?.()}
      >
        {t('project.leaveBody')}
      </Dialog>
    </>
  )
}
