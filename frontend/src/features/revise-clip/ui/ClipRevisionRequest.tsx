import { useState, type ReactNode } from 'react'
import { Send } from 'lucide-react'
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
import { Popover, ProgressBar, SegmentedControl, Textarea, Typography } from '@/shared/ui'
import { useClipRevision } from '../api/useClipRevision'

/** ②'s dock, in the post editor's shape (CLIP-40 →POST-45): the field's own heading row with the
 *  step's actions at its right, then one field and one send control. The target — footage flow,
 *  captions or both — is chosen where the request is approved, so the row the owner types in
 *  holds nothing but the request. */
export function ClipRevisionRequest({
  ownerId,
  project,
  observe,
  write,
  job,
  disabled = false,
  flush,
  cancelAction,
  action,
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
  /** Rendered at the top-right of the row's heading — 렌더하기 and, once a render exists,
   *  확정하기. A slot, because a feature may not reach a sibling feature (ARCH-13). */
  action?: ReactNode
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
    <div className="min-w-0 space-y-2">
      <div className="flex flex-wrap items-center justify-between gap-2">
        {/* The field's VISIBLE name, and a plain heading while the run replaces the field: a label
            pointing at a control that is not there names nothing. */}
        {running ? (
          <Typography variant="fieldTitle" as="p" className="min-w-0">
            {t('revision.request')}
          </Typography>
        ) : (
          <Typography
            variant="fieldTitle"
            as="label"
            htmlFor="clip-revision-request"
            className="min-w-0"
          >
            {t('revision.request')}
          </Typography>
        )}
        {action && (
          <div className="flex min-w-0 flex-1 flex-wrap items-center justify-end gap-2">
            {action}
          </div>
        )}
      </div>
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
        <fieldset disabled={disabled} className="min-w-0 space-y-1">
          <div className="flex min-w-0 items-end gap-2">
            <Textarea
              id="clip-revision-request"
              inputMode="text"
              aria-describedby={request ? 'clip-revision-count' : undefined}
              rows={1}
              className="max-h-24 min-w-0 flex-1"
              autoGrow
              value={request}
              onChange={(event) =>
                setRequest(boundedText(event.target.value, CLIP_PROJECT_LIMITS.instruction))
              }
            />
            <Popover
              label={t('revision.send')}
              triggerLabel={<Send aria-hidden="true" className="size-5" />}
              triggerSize="icon"
              triggerVariant="cta"
              className="shrink-0"
              placement="above"
              align="end"
              phone="sheet"
              disabled={disabled || !request.trim()}
            >
              {(close) => (
                <div className="space-y-3">
                  {/* Sending chooses what the request is about, then approves what it costs: the
                      priced calls follow the target (CLIP-132), so the two are read together. */}
                  <div className="space-y-1">
                    <Typography variant="label" as="p" id="clip-revision-target-label">
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
                    onApprove={(quote) => {
                      void revision.start(quote, flush)
                      close()
                    }}
                  />
                </div>
              )}
            </Popover>
          </div>
          {/* The counter only once there is something to count: the dock stands over the
              timeline the whole time, so an idle row is height taken from it (§0). */}
          {request && (
            <Typography variant="meta" as="p" id="clip-revision-count">
              {t('revision.count', { used, max: CLIP_PROJECT_LIMITS.instruction })}
            </Typography>
          )}
        </fieldset>
      )}
      {revision.cancelled && (
        <Typography variant="body" role="status">
          {t('revision.cancelled')}
        </Typography>
      )}
      <ClipFailureNotice failure={revision.failure} />
    </div>
  )
}
