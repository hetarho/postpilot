import i18next from 'i18next'
import type { ClipProject } from './types'

/** What a clip project IS right now, from what it already stores (CLIP-36, CLIP-226). Nothing new
 *  is persisted for it, so the directory row, the step bar and the status line cannot disagree —
 *  and the list, which never carries `editing` (the proto says detail only), derives the same
 *  values as the detail does. */
export type ClipState = 'draft' | 'refining' | 'confirmed' | 'completed' | 'published'

type StateProject = Pick<
  ClipProject,
  'editPlanRevision' | 'renderedPlanRevision' | 'result' | 'finalized'
> &
  Partial<Pick<ClipProject, 'storyline' | 'confirmed' | 'publishedUrl'>>

/** Finalization is irreversible and outranks everything: with a saved 게시 URL the clip is
 *  게시됨, without one 완료 — which is also how a project finalized before confirmation existed
 *  reads (CLIP-76, CLIP-232). Short of that, 확정 holds exactly while the plan is the confirmed
 *  revision (CLIP-229), and a storyline, a plan or a render is ② (CLIP-36, CLIP-177). */
export function clipState(project: StateProject): ClipState {
  if (project.finalized) return project.publishedUrl ? 'published' : 'completed'
  if (clipConfirmed(project)) return 'confirmed'
  if (project.editPlanRevision > 0 || project.result || project.storyline) return 'refining'
  return 'draft'
}

/** CLIP-229's reversible 확정: the project is confirmed exactly while its plan is the revision
 *  확정하기 recorded, so the first changed plan save returns it to 다듬는 중 without a write. */
export function clipConfirmed(
  project: Pick<ClipProject, 'editPlanRevision'> & Partial<Pick<ClipProject, 'confirmed'>>,
): boolean {
  return (
    project.editPlanRevision > 0 && project.confirmed?.planRevision === project.editPlanRevision
  )
}

/** 확정, 완료 and 게시됨 are the clip's finished states: 완성 holds them (CLIP-41, THEME-73) and
 *  the workspace opens them on ③ (CLIP-36). */
export function clipFinished(state: ClipState): boolean {
  return state === 'confirmed' || state === 'completed' || state === 'published'
}

/** The one vocabulary for that state. `i18next.t` directly rather than a hook: this layer imports
 *  no react (ARCH-18), and `entities/model-catalog`'s `levelPrefix` reads the same way. */
export function clipStateLabel(state: ClipState): string {
  return i18next.t(`state.${state}`, { ns: 'clips' })
}
