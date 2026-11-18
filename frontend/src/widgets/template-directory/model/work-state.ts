import type { AuthoringSummary } from '@/entities/ai-authoring'

/** The one work state a directory tile may carry, in TMPL-24's priority: AI working, then a
 *  publication to confirm, then a conflict, then unpublished changes. A saved template with none
 *  shows nothing — being saved is not news (THEME-76). */
export type TemplateWorkState = 'activeAI' | 'publicationPending' | 'conflict' | 'unpublished'

export function templateWorkState(summary?: AuthoringSummary): TemplateWorkState | null {
  if (!summary) return null
  if (summary.activeJobId) return 'activeAI'
  if (summary.publicationPending) return 'publicationPending'
  if (summary.targetConflict) return 'conflict'
  if (summary.hasUnpublishedChanges) return 'unpublished'
  return null
}
