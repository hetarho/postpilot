import i18next from 'i18next'
import { isTerminal } from '@/entities/generation-job/@x/clip-project'
import type { ClipProject } from './types'
import { clipState } from './state'

/** History distinguishes a prepared story from a built video without changing lifecycle steps,
 *  then names 확정, 완료 and 게시됨 apart (CLIP-226). */
export const CLIP_HISTORY_STATES = [
  'draft',
  'storyline_ready',
  'refining',
  'confirmed',
  'completed',
  'published',
] as const
export type ClipHistoryState = (typeof CLIP_HISTORY_STATES)[number]

/** The status filter's choices besides 전체: 완료 holds 게시됨 rows, so a published clip is
 *  never hidden by choosing 완료 (CLIP-226). */
export const CLIP_HISTORY_FILTERS = [
  'draft',
  'storyline_ready',
  'refining',
  'confirmed',
  'completed',
] as const
export type ClipHistoryFilter = (typeof CLIP_HISTORY_FILTERS)[number]

type HistoryProject = Parameters<typeof clipState>[0] & {
  latestJob?: Pick<NonNullable<ClipProject['latestJob']>, 'status' | 'cancelRequestedAt'>
}
type HistoryTone = 'neutral' | 'accent' | 'success' | 'done' | 'info' | 'danger'

export function clipHistoryState(project: Parameters<typeof clipState>[0]): ClipHistoryState {
  if (!project.finalized && project.storyline && !project.editPlanRevision && !project.result)
    return 'storyline_ready'
  return clipState(project)
}

/** Whether a project's history state falls under a filter value. The filter reads the project's
 *  OWN state, never the latest job's badge, so a draft mid-generation stays under 초안. Existing
 *  `refining` URLs include every editable ② project, a prepared story among them. */
export function clipMatchesHistoryFilter(
  project: Parameters<typeof clipState>[0],
  filter: ClipHistoryFilter,
): boolean {
  if (filter === 'storyline_ready') return clipHistoryState(project) === 'storyline_ready'
  const state = clipState(project)
  return filter === 'completed' ? state === 'completed' || state === 'published' : state === filter
}

/** A filter value from an address or a stored return context. The filter's former `finished`
 *  value selects 완료; anything else unknown is dropped rather than refused. */
export function clipHistoryFilter(value: unknown): ClipHistoryFilter | undefined {
  if (value === 'finished') return 'completed'
  return CLIP_HISTORY_FILTERS.includes(value as ClipHistoryFilter)
    ? (value as ClipHistoryFilter)
    : undefined
}

export function clipHistoryStateLabel(state: ClipHistoryState): string {
  return i18next.t(`state.${state}`, { ns: 'clips' })
}

/** Read the way a post's badge reads: 확정 as 확정, a finalized clip as a post that went up. */
const TONE: Record<ClipHistoryState, HistoryTone> = {
  draft: 'neutral',
  storyline_ready: 'accent',
  refining: 'accent',
  confirmed: 'success',
  completed: 'done',
  published: 'done',
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
  return { label: clipHistoryStateLabel(state), tone: TONE[state] }
}
