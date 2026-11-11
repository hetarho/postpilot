import i18next from 'i18next'
import { isTerminal } from '@/entities/generation-job/@x/clip-project'
import type { ClipProject } from './types'
import { clipState } from './state'

/** History distinguishes a prepared story from a built video without changing lifecycle steps. */
export const CLIP_HISTORY_STATES = ['draft', 'storyline_ready', 'refining', 'finished'] as const
export type ClipHistoryState = (typeof CLIP_HISTORY_STATES)[number]
type HistoryProject = Parameters<typeof clipState>[0] & {
  latestJob?: Pick<NonNullable<ClipProject['latestJob']>, 'status' | 'cancelRequestedAt'>
}
type HistoryTone = 'neutral' | 'accent' | 'success' | 'info' | 'danger'

export function clipHistoryState(project: Parameters<typeof clipState>[0]): ClipHistoryState {
  if (!project.finalized && project.storyline && !project.editPlanRevision && !project.result)
    return 'storyline_ready'
  return clipState(project)
}

export function clipHistoryStateLabel(state: ClipHistoryState): string {
  return i18next.t(`state.${state}`, { ns: 'clips' })
}

/** Both channel histories and the aggregate give the latest attempt the same precedence. */
export function clipHistoryStatus(project: HistoryProject): { label: string; tone: HistoryTone } {
  const job = project.latestJob
  if (job && !isTerminal(job))
    return {
      label: i18next.t(job.cancelRequestedAt ? 'cancellation.cancelling' : 'state.generating', {
        ns: 'clips',
      }),
      tone: 'info',
    }
  if (job?.status === 'failed')
    return { label: i18next.t('state.failed', { ns: 'clips' }), tone: 'danger' }
  if (job?.status === 'cancelled')
    return { label: i18next.t('cancellation.cancelled', { ns: 'clips' }), tone: 'neutral' }
  const state = clipHistoryState(project)
  return {
    label: clipHistoryStateLabel(state),
    tone: state === 'finished' ? 'success' : state === 'draft' ? 'neutral' : 'accent',
  }
}
