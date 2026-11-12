import { useState, type ReactNode } from 'react'
import { SendHorizontal, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  CLIP_PROJECT_LIMITS,
  ClipFailureNotice,
  ClipQuoteApproval,
  boundedFreeText,
  freeTextLength,
  type ClipProject,
} from '@/entities/clip-project'
import { progressLabel, progressRatio, type GenerationJob } from '@/entities/generation-job'
import type { ModelRef } from '@/entities/model-catalog'
import { useMyPlan } from '@/entities/plan'
import { Button, Popover, ProgressBar, Textarea, Typography } from '@/shared/ui'
import { useClipRevision } from '../api/useClipRevision'
import { SaveAsVideoGuidelineButton } from './SaveAsVideoGuidelineButton'

/** The composer of ②'s step bar, in the shape of the post's 글 다듬기 field (CLIP-40 →POST-56,
 *  `features/edit-with-ai`'s `ReviseForm`): the field's visible heading IS its label, then the
 *  request with an icon-only send, and the count once there is something to count.
 *
 *  The owner names no target: the request decides whether the footage flow, the narration or
 *  both are rewritten (CLIP-131), so sending opens nothing but the approval of the two writing
 *  calls it may need (CLIP-132). A selected scene block scopes it, shown as a removable chip above
 *  the field; without one the request is about the whole clip. */
export function ClipRevisionRequest({
  ownerId,
  project,
  observe,
  write,
  job,
  disabled = false,
  flush,
  cancelAction,
  scope,
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
  /** The selected scene block (1-based) and the way to drop it from the request. */
  scope?: { cut: number; clear: () => void }
}) {
  const { t } = useTranslation('clips')
  const [request, setRequest] = useState('')
  const { myPlan } = useMyPlan()
  const revision = useClipRevision({
    ownerId,
    project,
    request,
    scopeCut: scope?.cut,
    observe,
    write,
    job,
  })
  const used = freeTextLength(request)
  const running = revision.running
  const progress = revision.job ? progressRatio(revision.job) : undefined
  const title = revision.job ? progressLabel(revision.job) : t('revision.running')
  return (
    <div className="grid min-w-0 gap-2">
      {/* The field's VISIBLE name at the post's `fieldTitle` role, and a plain heading while the
          run replaces the field: a label pointing at a control that is not there names nothing. */}
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
        <fieldset disabled={disabled} className="grid min-w-0 gap-2">
          {scope && (
            <div className="flex min-w-0 flex-wrap gap-2">
              <Button
                type="button"
                variant="secondary"
                size="compact"
                className="min-h-11"
                aria-label={t('revision.scopeRemove', { number: scope.cut })}
                onClick={scope.clear}
              >
                {t('revision.scope', { number: scope.cut })}
                <X className="size-3.5" aria-hidden />
              </Button>
            </div>
          )}
          <div className="flex min-w-0 items-end gap-2">
            <Textarea
              id="clip-revision-request"
              inputMode="text"
              autoComplete="off"
              autoCapitalize="sentences"
              enterKeyHint="enter"
              aria-describedby={request ? 'clip-revision-count' : undefined}
              rows={1}
              autoGrow
              value={request}
              onChange={(event) =>
                setRequest(boundedFreeText(event.target.value, CLIP_PROJECT_LIMITS.instruction))
              }
              className="max-h-field min-w-0 flex-1"
            />
            {/* Sending approves what the request may cost before anything runs (QUOTA-45). */}
            <Popover
              label={t('revision.send')}
              triggerLabel={<SendHorizontal aria-hidden="true" className="size-5" />}
              triggerSize="icon"
              triggerVariant="secondary"
              className="shrink-0"
              placement="above"
              align="end"
              phone="sheet"
              disabled={disabled || !request.trim()}
            >
              {(close) => (
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
              )}
            </Popover>
          </div>
          {/* The counter only once there is something to count: the bar stands over the scene
              blocks the whole time, so an idle row is height taken from them (THEME-8). */}
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
      {/* After the run THIS session started completes: its request, in the owner's own words,
          can become a standing 영상 지침 (GUIDE-45). */}
      {!running && revision.completedRequest && (
        <SaveAsVideoGuidelineButton
          ownerId={ownerId}
          request={revision.completedRequest}
          videoTemplateId={project.videoTemplateId}
        />
      )}
      <ClipFailureNotice failure={revision.failure} />
    </div>
  )
}
