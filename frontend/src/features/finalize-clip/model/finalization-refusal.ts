import type { ClipProject } from '@/entities/clip-project'

export function finalizationRefusal(project: ClipProject): ClipProject['finalizationRefusal'] {
  if (project.finalized) return 'finalized'
  if (project.finalizationRefusal) return project.finalizationRefusal
  if (project.latestJob && ['queued', 'running'].includes(project.latestJob.status)) return 'busy'
  if (!project.result?.id) return 'missing_render'
  if (project.renderedPlanRevision !== project.editPlanRevision) return 'stale_render'
  if (!project.canFinalize) return 'unavailable'
}
