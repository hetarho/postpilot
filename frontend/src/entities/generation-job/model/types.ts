import i18next from 'i18next'
import type { AppFailure, ContentLanguage } from '@/shared/api'

export interface ModelRef {
  providerId: string
  modelId: string
}

export interface GenerationJob {
  id: string
  kind: string
  status: string
  stage: string
  progressDone: number
  progressTotal: number
  failure: AppFailure | undefined
  postSlug: string
  observeModel: ModelRef | undefined
  writeModel: ModelRef | undefined
  createdAt: string
  updatedAt: string
  targetLanguage: ContentLanguage | undefined
}

export function isTerminal(job: Pick<GenerationJob, 'status'> | undefined): boolean {
  return job?.status === 'done' || job?.status === 'failed'
}

/** The stages that report a MEANINGFUL ratio. `write` and `analyze` are one provider call each —
 *  their done/total is 0/1 until the call returns, which is a bar that jumps from empty to full
 *  and says nothing on the way (see `progressRatio`). */
const RATIO_STAGES = new Set(['observe', 'compare_observe', 'compare_write', 'compare_analyze'])

/** WHICH STAGE is running, and nothing else. The numbers are the progress bar's value
 *  (`progressRatio`), so spelling them out here would print the same fact twice in two
 *  grammars — and in a container sized for a warning (change 15).
 *
 *  An unrecognized or not-yet-set stage is a running job like any other, so it takes the generic
 *  running label rather than announcing that nothing has happened yet. This is also why a voice
 *  `seed` run finally says something true: it hits this branch. */
export function progressLabel(job: Pick<GenerationJob, 'stage'>): string {
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
  job: Pick<GenerationJob, 'stage' | 'progressDone' | 'progressTotal'>,
): { done: number; total: number } | undefined {
  if (!RATIO_STAGES.has(job.stage) || job.progressTotal <= 0) return undefined
  return { done: job.progressDone, total: job.progressTotal }
}
