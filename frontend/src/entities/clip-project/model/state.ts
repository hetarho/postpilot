import i18next from 'i18next'
import type { ClipProject } from './types'

/** What a clip project IS right now, from what it already stores (CLIP-36). Nothing new is
 *  persisted for it, so the directory row, the step bar and the status line cannot disagree —
 *  and the list, which never carries `editing` (the proto says detail only), derives the same
 *  three values as the detail does. */
export type ClipState = 'draft' | 'refining' | 'finished'

/** Only explicit confirmation completes a project; a render stays editable. */
export function clipState(
  project: Pick<ClipProject, 'editPlanRevision' | 'renderedPlanRevision' | 'result' | 'finalized'>,
): ClipState {
  if (project.finalized) return 'finished'
  if (project.editPlanRevision > 0 || project.result) return 'refining'
  return 'draft'
}

/** The one vocabulary for that state. `i18next.t` directly rather than a hook: this layer imports
 *  no react (ARCH-18), and `entities/model-catalog`'s `levelPrefix` reads the same way. */
export function clipStateLabel(state: ClipState): string {
  return i18next.t(`state.${state}`, { ns: 'clips' })
}
