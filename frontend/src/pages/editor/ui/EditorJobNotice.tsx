import type { RefObject } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { FailureNotice } from '@/entities/generation-job'
import type { PostDraft } from '@/entities/post'
import type { GenerationActionsHandle } from '@/features/generate-post'
import type { ReviseFormHandle } from '@/features/edit-with-ai'
import type { EditorStep } from '../model/steps'
import type { EditorJobView } from '../model/useEditorJob'

/** A FAILURE, and only a failure. It stays in the dock while the job's progress moved to the
 *  page-top bar, because a failure carries a retry: something the user can act on is a control,
 *  not a status (change 15). Its presence is also what decides whether the bar exists at all on
 *  글 완성 — a bar holding nothing is chrome with nothing to say (§0).
 *
 *  The RETRY is offered only on the step that owns the job, because that is where the control it
 *  calls is mounted. */
export function EditorJobNotice({
  post,
  step,
  jobView,
  generateRef,
  reviseRef,
}: {
  post: PostDraft
  step: EditorStep
  jobView: EditorJobView
  generateRef: RefObject<GenerationActionsHandle | null>
  reviseRef: RefObject<ReviseFormHandle | null>
}) {
  const { t } = useTranslation('posts')
  const navigate = useNavigate()
  const { job, jobId, jobStep } = jobView
  return !jobId ? null : jobView.isError ? (
    <FailureNotice message={t('editor.jobLoadFailed')} onRetry={jobView.refetch} />
  ) : job?.status === 'failed' ? (
    <FailureNotice
      failure={job.failure}
      onRetry={
        step !== jobStep || (job.kind === 'revise' && jobView.startedStep === undefined)
          ? undefined
          : () =>
              job.kind === 'revise'
                ? reviseRef.current?.start()
                : job.kind === 'model_experiment'
                  ? post.pendingExperimentId
                    ? void navigate({
                        to: '/ai-models/experiments/$id',
                        params: { id: post.pendingExperimentId },
                      })
                    : undefined
                  : generateRef.current?.startGeneration()
      }
    />
  ) : null
}
