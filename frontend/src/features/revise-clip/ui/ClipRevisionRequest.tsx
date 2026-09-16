import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import {
  boundedText,
  ClipFailureNotice,
  ClipQuoteApproval,
  CLIP_PROJECT_LIMITS,
  CLIP_REVISION_TARGETS,
  type ClipProject,
  type ClipRevisionTarget,
} from '@/entities/clip-project'
import { compositionCharacters } from '@/entities/clip-template'
import { progressLabel, progressRatio, type GenerationJob } from '@/entities/generation-job'
import type { ModelRef } from '@/entities/model-catalog'
import { useMyPlan } from '@/entities/plan'
import { FieldLabel, ProgressBar, SegmentedControl, Textarea, Typography } from '@/shared/ui'
import { useClipRevision } from '../api/useClipRevision'

/** ②'s own panel asks the writer for a revision (CLIP-131, CLIP-40).
 *
 *  It is a panel control with its own approval rather than a dock action: THEME-39
 *  allows one ActionBar per step and ②'s is already full with 확정하기, 다시 렌더
 *  and the download. The owner stays here while it runs — the preview and the
 *  timeline are the thing being revised, and watching them is the point — so this
 *  path deliberately does not open CLIP-78's focused job view. */
export function ClipRevisionRequest({
  ownerId,
  project,
  observe,
  write,
  job,
  disabled = false,
  flush,
  cancelAction,
}: {
  ownerId: string
  project: ClipProject
  observe: ModelRef | null
  write: ModelRef | null
  /** The page's poll of the one clip job (CLIP-36), never a second one. */
  job?: GenerationJob
  /** Another clip job holds the project, or it is finalized. */
  disabled?: boolean
  /** Writes the owner's unsaved edits and answers with the plan revision. */
  flush: () => Promise<number>
  cancelAction?: ReactNode
}) {
  const { t } = useTranslation('clips')
  const [request, setRequest] = useState('')
  const [target, setTarget] = useState<ClipRevisionTarget>('narration')
  const { myPlan } = useMyPlan()
  const revision = useClipRevision({ ownerId, project, request, target, observe, write, job })
  const used = compositionCharacters(request)
  const running = revision.running
  const progress = revision.job ? progressRatio(revision.job) : undefined
  const title = revision.job ? progressLabel(revision.job) : t('revision.running')
  return (
    <section aria-labelledby="clip-revision-heading" className="mt-10 space-y-3">
      <Typography variant="title" id="clip-revision-heading">
        {t('revision.title')}
      </Typography>
      <Typography variant="body" className="text-content-secondary break-words">
        {t('revision.help')}
      </Typography>
      {running ? (
        <div className="space-y-3">
          <Typography variant="body" role="status" aria-live="polite">
            {title}
          </Typography>
          <ProgressBar label={title} done={progress?.done} total={progress?.total} />
          <Typography variant="meta" as="p">
            {t('revision.readOnly')}
          </Typography>
          {cancelAction}
        </div>
      ) : (
        <fieldset disabled={disabled} className="min-w-0 space-y-3">
          <div>
            <FieldLabel htmlFor="clip-revision-request">{t('revision.request')}</FieldLabel>
            <Textarea
              id="clip-revision-request"
              inputMode="text"
              autoGrow
              value={request}
              onChange={(event) =>
                setRequest(boundedText(event.target.value, CLIP_PROJECT_LIMITS.instruction))
              }
            />
            <Typography variant="meta" as="p" className="mt-2">
              {t('revision.count', { used, max: CLIP_PROJECT_LIMITS.instruction })}
            </Typography>
          </div>
          <div className="space-y-2">
            <Typography variant="fieldTitle" as="p">
              {t('revision.target')}
            </Typography>
            <SegmentedControl
              ariaLabel={t('revision.target')}
              value={target}
              options={CLIP_REVISION_TARGETS.map((value) => ({
                value,
                label: t(`revision.targets.${value}`),
              }))}
              onChange={setTarget}
            />
          </div>
          {revision.cancelled && (
            <Typography variant="body" role="status">
              {t('revision.cancelled')}
            </Typography>
          )}
          <ClipFailureNotice failure={revision.failure} />
          <ClipQuoteApproval
            quote={revision.quote}
            quoting={revision.quoting}
            expired={revision.expired}
            balance={myPlan?.balance}
            error={revision.error}
            approveDisabled={disabled || !myPlan || !request.trim()}
            approveLabel={
              revision.quote
                ? t('revision.approve', { amount: revision.quote.maxCredits })
                : t('revision.send')
            }
            onRefresh={revision.refresh}
            onApprove={(quote) => void revision.start(quote, flush)}
          />
        </fieldset>
      )}
    </section>
  )
}
