import i18next from 'i18next'
import type { ClipProject } from './types'

/** What a clip project IS right now, from what it already stores (CLIP-36). Nothing new is
 *  persisted for it, so the directory row, the step bar and the status line cannot disagree —
 *  and the list, which never carries `editing` (the proto says detail only), derives the same
 *  three values as the detail does. */
export type ClipState = 'draft' | 'refining' | 'finished'

/** `editPlanRevision === 0` IS "no editing state": the server rejects `revision <= 0` everywhere
 *  (`clip/rerender.go`, `clip/store/correction.go`) and only a successful generation writes the
 *  first plan. Anything between a written plan and a matching render is being refined, which
 *  covers both an unsaved correction's successor and a plan whose render failed.
 *
 *  The finished test runs FIRST so a project that HAS a rendered result can never be called a
 *  draft: ① is the step that produces one and holds no way to see it, so a state that hid an
 *  existing result behind it would strand the owner's only copy. */
export function clipState(
  project: Pick<ClipProject, 'editPlanRevision' | 'renderedPlanRevision' | 'result'>,
): ClipState {
  if (project.result && project.editPlanRevision === project.renderedPlanRevision) return 'finished'
  if (project.editPlanRevision <= 0) return 'draft'
  return 'refining'
}

/** The one vocabulary for that state. `i18next.t` directly rather than a hook: this layer imports
 *  no react (ARCH-18), and `entities/model-catalog`'s `levelPrefix` reads the same way. */
export function clipStateLabel(state: ClipState): string {
  return i18next.t(`state.${state}`, { ns: 'clips' })
}
