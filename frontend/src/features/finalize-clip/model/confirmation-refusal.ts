import type { ClipProject } from '@/entities/clip-project'

export type ClipConfirmationRefusal = NonNullable<ClipProject['confirmationRefusal']>

/** Why 확정하기 cannot confirm, read from the saved project (CLIP-75). `unsaved` says the draft
 *  holds edits the press saves first: the server judged an older plan, so its verdict on the plan
 *  itself (an invalid plan, narration awaiting DUB-26's regeneration) is left to the press. */
export function confirmationRefusal(
  project: ClipProject,
  { unsaved = false }: { unsaved?: boolean } = {},
): ClipConfirmationRefusal | undefined {
  if (project.finalized) return 'finalized'
  if (project.latestJob && ['queued', 'running'].includes(project.latestJob.status)) return 'busy'
  const refusal = project.confirmationRefusal
  if (refusal === 'finalized' || refusal === 'busy') return refusal
  if (unsaved) return
  if (refusal) return refusal
  if (project.canConfirm === false) return 'unavailable'
}
