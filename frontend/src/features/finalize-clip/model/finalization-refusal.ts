import { clipConfirmed, type ClipProject } from '@/entities/clip-project'

/** Why ③'s 게시 URL and `완료로 표시` cannot finalize (CLIP-76, CLIP-232): the server's verdict
 *  first, then what this read already shows — a job running, no confirmed plan, no durable render
 *  matching it. */
export function finalizationRefusal(project: ClipProject): ClipProject['finalizationRefusal'] {
  if (project.finalized) return 'finalized'
  if (project.finalizationRefusal) return project.finalizationRefusal
  if (project.latestJob && ['queued', 'running'].includes(project.latestJob.status)) return 'busy'
  if (!clipConfirmed(project)) return 'unconfirmed'
  if (!project.result?.id) return 'missing_render'
  if (project.renderedPlanRevision !== project.editPlanRevision) return 'stale_render'
  if (!project.canFinalize) return 'unavailable'
}
