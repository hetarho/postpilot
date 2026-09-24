import i18next from 'i18next'
import type { AppFailure, ContentLanguage } from '@/shared/api'

export interface ModelRef {
  providerId: string
  modelId: string
}

export interface GenerationJob {
  cancelRequestedAt?: string
  cancellationPolicyVersion?: number
  canCancel?: boolean
  id: string
  kind: string
  status: string
  stage: string
  progressDone: number
  progressTotal: number
  failure: AppFailure | undefined
  postSlug: string
  clipProjectId?: string
  observeModel: ModelRef | undefined
  writeModel: ModelRef | undefined
  createdAt: string
  updatedAt: string
  targetLanguage: ContentLanguage | undefined
}

export function isTerminal(job: Pick<GenerationJob, 'status'> | undefined): boolean {
  return job?.status === 'done' || job?.status === 'failed' || job?.status === 'cancelled'
}

/** The stages that report a MEANINGFUL ratio. `write` and `analyze` are one provider call each —
 *  their done/total is 0/1 until the call returns, which is a bar that jumps from empty to full
 *  and says nothing on the way (see `progressRatio`). */
const RATIO_STAGES = new Set(['observe', 'compare_observe', 'compare_write', 'compare_analyze'])

/** The clip jobs: they report the clip stages and are read in the clips
 *  namespace. A revision is one of them — it makes the same writing calls a
 *  generation does, just over a plan that already exists (CLIP-131). */
const CLIP_KINDS = new Set(['generate_clip', 'render_clip', 'revise_clip'])

/** WHICH STAGE is running, and nothing else. The numbers are the progress bar's value
 *  (`progressRatio`), so spelling them out here would print the same fact twice in two
 *  grammars — and in a container sized for a warning (change 15).
 *
 *  An unrecognized or not-yet-set stage is a running job like any other, so it takes the generic
 *  running label rather than announcing that nothing has happened yet. This is also why a voice
 *  `seed` run finally says something true: it hits this branch. */
const MEDIA_WAIT_STAGES = ['prepare_wait', 'prepare_retry', 'render_wait', 'render_retry'] as const
const GENERATION_CLIP_STAGES = [
  'prepare',
  'analyze',
  'analyze_retry',
  'flow',
  'flow_retry',
  'narrate',
  'narrate_retry',
  // The single writing call this build no longer makes: a job queued before it
  // split into two still shows a stage the owner can read.
  'plan',
  'plan_retry',
  'render',
  'save',
  'cleanup',
] as const
export const CLIP_STAGES = [...MEDIA_WAIT_STAGES, ...GENERATION_CLIP_STAGES] as const

export function progressLabel(
  job: Pick<GenerationJob, 'stage'> &
    Partial<
      Pick<
        GenerationJob,
        'kind' | 'progressDone' | 'progressTotal' | 'cancelRequestedAt' | 'status'
      >
    >,
): string {
  if (CLIP_KINDS.has(job.kind ?? '')) {
    if (job.cancelRequestedAt && !['done', 'failed', 'cancelled'].includes(job.status ?? '')) {
      return i18next.t('cancellation.cancelling', { ns: 'clips' })
    }
    const stage = GENERATION_CLIP_STAGES.find((stage) => stage === job.stage)
    const media = MEDIA_WAIT_STAGES.find((stage) => stage === job.stage)
    const label = i18next.t(
      media ? `mediaStage.${media}` : stage ? `generation.stage.${stage}` : 'generation.running',
      { ns: 'clips' },
    )
    if (
      ['analyze_retry', 'flow_retry', 'narrate_retry', 'plan_retry'].includes(job.stage) &&
      job.progressTotal === 3 &&
      job.progressDone &&
      job.progressDone >= 1 &&
      job.progressDone <= 3
    ) {
      return i18next.t('generation.responseRetryCount', {
        ns: 'clips',
        stage: label,
        current: job.progressDone,
        total: job.progressTotal,
      })
    }
    return label
  }
  switch (job.stage) {
    case 'observe':
      return i18next.t('generation.observing', { ns: 'posts' })
    case 'write':
      return i18next.t('generation.writing', { ns: 'posts' })
    case 'analyze':
      return i18next.t('generation.analyzing', { ns: 'posts' })
    case 'compare_write':
      return i18next.t('generation.compareWriting', { ns: 'posts' })
    case 'compare_observe':
      return i18next.t('generation.compareObserving', { ns: 'posts' })
    case 'compare_analyze':
      return i18next.t('generation.compareAnalyzing', { ns: 'posts' })
    default:
      return i18next.t('generation.running', { ns: 'posts' })
  }
}

/** The bar's value, or `undefined` where this stage has no ratio worth drawing — which is what
 *  puts the same 2px track into its indeterminate state instead of a second control. A total of
 *  zero is also `undefined`: 0/0 is not "complete". */
export function progressRatio(
  job: Pick<GenerationJob, 'stage' | 'progressDone' | 'progressTotal'> &
    Partial<Pick<GenerationJob, 'kind' | 'cancelRequestedAt'>>,
): { done: number; total: number } | undefined {
  if (job.cancelRequestedAt) return undefined
  const clipRatio = CLIP_KINDS.has(job.kind ?? '') && ['prepare', 'analyze'].includes(job.stage)
  if ((!RATIO_STAGES.has(job.stage) && !clipRatio) || job.progressTotal <= 0) return undefined
  return { done: job.progressDone, total: job.progressTotal }
}
