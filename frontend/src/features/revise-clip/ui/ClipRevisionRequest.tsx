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
import {
  FieldLabel,
  Popover,
  ProgressBar,
  SegmentedControl,
  Textarea,
  Typography,
} from '@/shared/ui'
import { useClipRevision } from '../api/useClipRevision'

/** The dock's composer keeps the plan visible during both approval and revision. */
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
    <div className="min-w-0">
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
        <fieldset
          disabled={disabled}
          className="flex min-w-0 flex-wrap items-end gap-2 sm:flex-nowrap"
        >
          <div className="w-full min-w-0 sm:order-2 sm:flex-1">
            <FieldLabel htmlFor="clip-revision-request">{t('revision.request')}</FieldLabel>
            <Textarea
              id="clip-revision-request"
              inputMode="text"
              aria-describedby="clip-revision-count"
              rows={1}
              className="max-h-24"
              autoGrow
              value={request}
              onChange={(event) =>
                setRequest(boundedText(event.target.value, CLIP_PROJECT_LIMITS.instruction))
              }
            />
            <Typography variant="meta" as="p" id="clip-revision-count" className="mt-1">
              {t('revision.count', { used, max: CLIP_PROJECT_LIMITS.instruction })}
            </Typography>
          </div>
          <div className="min-w-0 flex-1 sm:order-1 sm:flex-none">
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
          <Popover
            label={t('revision.send')}
            triggerLabel={<Send aria-hidden="true" className="size-5" />}
            triggerSize="icon"
            triggerVariant="cta"
            className="shrink-0 sm:order-3"
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
