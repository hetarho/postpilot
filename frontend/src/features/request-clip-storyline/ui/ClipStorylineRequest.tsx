import { useState } from 'react'
import { Send } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  CLIP_PROJECT_LIMITS,
  ClipFailureNotice,
  ClipQuoteApproval,
  boundedText,
  type ClipProject,
} from '@/entities/clip-project'
import { compositionCharacters } from '@/entities/clip-template'
import { progressLabel, progressRatio, type GenerationJob } from '@/entities/generation-job'
import type { ModelRef } from '@/entities/model-catalog'
import { useMyPlan } from '@/entities/plan'
import { Popover, ProgressBar, Textarea, Typography } from '@/shared/ui'
import { useClipStorylineRequest } from '../api/useClipStorylineRequest'

/** The storyline space's AI request (CLIP-181): one field and a send control that opens its own
 *  approval — ②'s revision composer's shape with no target to choose, since the storyline is the
 *  one document it rewrites. While the request runs, its progress stands where the field was. */
export function ClipStorylineRequest({
  ownerId,
  project,
  observe,
  write,
  job,
  disabled = false,
}: {
  ownerId: string
  project: ClipProject
  observe: ModelRef | null
  write: ModelRef | null
  /** The page's poll of the one clip job, never a second one. */
  job?: GenerationJob
  /** Another clip job holds the project. */
  disabled?: boolean
}) {
  const { t } = useTranslation('clips')
  const [request, setRequest] = useState('')
  const { myPlan } = useMyPlan()
  const storyline = useClipStorylineRequest({ ownerId, project, request, observe, write, job })
  const running = storyline.running
  const title = storyline.job ? progressLabel(storyline.job) : t('storylineRequest.running')
  const progress = storyline.job ? progressRatio(storyline.job) : undefined
  return (
    <div className="mt-2 min-w-0 space-y-1">
      {running ? (
        <div className="space-y-2">
          <Typography variant="body" role="status" aria-live="polite">
            {title}
          </Typography>
          <ProgressBar label={title} done={progress?.done} total={progress?.total} />
        </div>
      ) : (
        <fieldset disabled={disabled} className="min-w-0 space-y-1">
          <Typography variant="fieldTitle" as="label" htmlFor="clip-storyline-request">
            {t('storylineRequest.label')}
          </Typography>
          <div className="flex min-w-0 items-end gap-2">
            <Textarea
              id="clip-storyline-request"
              rows={1}
              autoGrow
              className="max-h-24 min-w-0 flex-1"
              value={request}
              onChange={(event) =>
                setRequest(boundedText(event.target.value, CLIP_PROJECT_LIMITS.instruction))
              }
            />
            <Popover
              label={t('storylineRequest.send')}
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
                <ClipQuoteApproval
                  quote={storyline.quote}
                  quoting={storyline.quoting}
                  expired={storyline.expired}
                  balance={myPlan?.balance}
                  error={storyline.error}
                  approveDisabled={disabled || !myPlan || !request.trim()}
                  approveLabel={
                    storyline.quote
                      ? t('storylineRequest.approve', { amount: storyline.quote.maxCredits })
                      : t('storylineRequest.send')
                  }
                  onRefresh={storyline.refresh}
                  onApprove={(quote) => {
                    void storyline.start(quote)
                    close()
                  }}
                />
              )}
            </Popover>
          </div>
          {request && (
            <Typography variant="meta" as="p">
              {t('storylineRequest.count', {
                used: compositionCharacters(request),
                max: CLIP_PROJECT_LIMITS.instruction,
              })}
            </Typography>
          )}
        </fieldset>
      )}
      <ClipFailureNotice failure={storyline.failure} />
    </div>
  )
}
