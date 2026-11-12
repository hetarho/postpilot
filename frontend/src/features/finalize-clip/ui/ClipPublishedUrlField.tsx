import { useId, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { ClipNoticeList, type ClipProject } from '@/entities/clip-project'
import { formatAppFailure } from '@/shared/lib'
import {
  AppFailureMessage,
  Button,
  Dialog,
  FieldLabel,
  FieldMessage,
  TextField,
  Typography,
} from '@/shared/ui'
import type { useFinalizeClip } from '../model/useFinalizeClip'
import { finalizationRefusal } from '../model/finalization-refusal'
import { validClipPublishedUrl } from '../model/published-url'

/** The consequences and unresolved notices belong to the confirmation itself (CLIP-76). */
export function FinalizeClipNotices({ project }: { project: ClipProject }) {
  const { t } = useTranslation('clips')
  return (
    <div className="min-w-0 space-y-3">
      <Typography variant="body">{t('finalization.notice')}</Typography>
      <ClipNoticeList
        notices={project.notices}
        language={project.language}
        cuts={project.editing?.plan.cuts}
        withTargets
      />
    </div>
  )
}

/** ③'s foot (CLIP-230, CLIP-232): the clip's 게시 URL with `완료로 표시` beside it, as the post's
 *  발행 URL stands at its ③'s foot (POST-54). Saving a URL finalizes as 게시됨 and `완료로 표시` as
 *  완료; both flush, read the saved plan and open ONE confirmation before anything ends. A
 *  finalized clip keeps the field, where a saved URL replaces the old one and changes nothing
 *  else. */
export function ClipPublishedUrlField({
  action,
  project,
  disabled,
  localRefusal,
}: {
  action: ReturnType<typeof useFinalizeClip>
  project: ClipProject
  disabled: boolean
  localRefusal?: ClipProject['finalizationRefusal']
}) {
  const { t } = useTranslation('clips')
  const headingId = useId()
  return (
    <section aria-labelledby={headingId} className="mt-10 min-w-0">
      <Typography variant="title" id={headingId}>
        {t('publishedUrl.title')}
      </Typography>
      {/* Keyed on the stored address, so every answer reseeds the field. */}
      <PublishedUrlForm
        key={project.publishedUrl ?? ''}
        action={action}
        project={project}
        disabled={disabled}
        localRefusal={localRefusal}
      />
    </section>
  )
}

function PublishedUrlForm({
  action,
  project,
  disabled,
  localRefusal,
}: {
  action: ReturnType<typeof useFinalizeClip>
  project: ClipProject
  disabled: boolean
  localRefusal?: ClipProject['finalizationRefusal']
}) {
  const { t } = useTranslation('clips')
  const id = useId()
  const finalized = !!project.finalized
  const channel = project.channel ?? 'naver_clip'
  const [draft, setDraft] = useState(project.publishedUrl ?? '')
  const [localError, setLocalError] = useState('')
  // Which press the last failure answers: a URL's refusal belongs under the field.
  const [intent, setIntent] = useState<'url' | 'complete'>()
  const [prepared, setPrepared] = useState<{ project: ClipProject; url?: string }>()
  const [saved, setSaved] = useState(false)
  const refusal = finalized
    ? undefined
    : (localRefusal ?? finalizationRefusal(project) ?? (disabled ? 'busy' : undefined))
  const held = disabled || action.uncertain || !!refusal
  const fieldError =
    localError || (intent === 'url' && action.failure ? formatAppFailure(action.failure) : '')
  const open =
    !!prepared &&
    !refusal &&
    prepared.project.editPlanRevision === project.editPlanRevision &&
    prepared.project.result?.id === project.result?.id

  const finalize = (url?: string) => {
    setIntent(url === undefined ? 'complete' : 'url')
    void action.prepare().then((saved) => {
      if (saved && !finalizationRefusal(saved)) setPrepared({ project: saved, url })
    })
  }

  const submit = (event: FormEvent) => {
    event.preventDefault()
    if (action.pending || (finalized ? disabled || action.uncertain : held)) return
    const trimmed = draft.trim()
    if (trimmed === '') return
    if (!validClipPublishedUrl(channel, trimmed)) {
      setLocalError(formatAppFailure({ reason: 'CLIP_PUBLISHED_URL_INVALID', params: {} }))
      return
    }
    if (!finalized) return finalize(trimmed)
    setIntent('url')
    void action.savePublishedUrl(trimmed).then(setSaved)
  }

  const reasonId = `${id}-reason`
  const errorId = `${id}-error`
  return (
    // `noValidate`: the browser's own url check would answer in its own words before ours.
    <form onSubmit={submit} noValidate className="mt-3 space-y-3">
      <div>
        <FieldLabel htmlFor={`${id}-url`}>{t(`publishedUrl.label.${channel}`)}</FieldLabel>
        <TextField
          id={`${id}-url`}
          type="url"
          inputMode="url"
          autoComplete="url"
          autoCapitalize="none"
          autoCorrect="off"
          enterKeyHint="done"
          spellCheck={false}
          value={draft}
          placeholder={t(`publishedUrl.placeholder.${channel}`)}
          aria-invalid={fieldError ? true : undefined}
          aria-describedby={
            [refusal ? reasonId : '', fieldError ? errorId : ''].filter(Boolean).join(' ') ||
            undefined
          }
          onChange={(event) => {
            setDraft(event.target.value)
            setLocalError('')
            setSaved(false)
          }}
          className="mt-1"
        />
        {fieldError && (
          <FieldMessage id={errorId} role="alert" className="mt-2">
            {fieldError}
          </FieldMessage>
        )}
        {saved && (
          <Typography variant="label" as="p" role="status" className="mt-2">
            {t('publishedUrl.saved')}
          </Typography>
        )}
      </div>
      {refusal && (
        <Typography variant="meta" as="p" id={reasonId}>
          {t(`finalization.refusal.${refusal}`)}
        </Typography>
      )}
      <div className="flex flex-wrap items-center gap-2">
        <Button
          type="submit"
          variant={finalized ? 'cta' : 'secondary'}
          pending={action.pending && intent === 'url'}
          disabled={finalized ? disabled || action.uncertain : held}
          aria-describedby={refusal ? reasonId : undefined}
        >
          {t(finalized ? 'publishedUrl.replace' : 'publishedUrl.save')}
        </Button>
        {!finalized && (
          <Button
            type="button"
            variant="cta"
            pending={action.pending && intent === 'complete'}
            disabled={held}
            aria-describedby={refusal ? reasonId : undefined}
            onClick={() => finalize()}
          >
            {t('finalization.confirm')}
          </Button>
        )}
      </div>
      {action.failure && intent !== 'url' && (
        <div role="alert">
          <AppFailureMessage failure={action.failure} />
        </div>
      )}
      {action.uncertain && (
        <div className="space-y-2">
          <Typography variant="body">{t('finalization.uncertain')}</Typography>
          <Button
            variant="secondary"
            pending={action.pending}
            onClick={() => void action.checkAgain()}
          >
            {t('project.retry')}
          </Button>
        </div>
      )}
      <Dialog
        open={open}
        title={t(prepared?.url ? 'publishedUrl.dialogTitle' : 'finalization.dialogTitle')}
        confirmLabel={t(prepared?.url ? 'publishedUrl.dialogConfirm' : 'finalization.confirm')}
        pending={action.pending}
        onClose={() => setPrepared(undefined)}
        onConfirm={() => {
          void action.confirm(prepared?.url).finally(() => setPrepared(undefined))
        }}
      >
        {prepared && <FinalizeClipNotices project={prepared.project} />}
      </Dialog>
    </form>
  )
}
