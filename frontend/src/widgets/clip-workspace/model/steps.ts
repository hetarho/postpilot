import { clipState, type ClipProject, type ClipState } from '@/entities/clip-project'
import { LIFECYCLE_STATIONS, lifecycleStationLabel, type LifecycleStation } from '@/shared/config'

export type ClipStep = LifecycleStation

/** The project's lifecycle as the five shared stations (CLIP-36, THEME-39). One list, so the bar
 *  and the panels cannot drift, and the same short names the post editor runs (→POST-44). */
export function clipSteps(): ReadonlyArray<{ value: ClipStep; label: string }> {
  return LIFECYCLE_STATIONS.map((value) => ({ value, label: lifecycleStationLabel(value) }))
}

export function clipStepLabel(step: ClipStep): string {
  return lifecycleStationLabel(step)
}

const BY_STATE: Record<Exclude<ClipState, 'draft' | 'refining'>, ClipStep> = {
  confirmed: 'finish',
  completed: 'finish',
  published: 'finish',
}

/** The step a project's persisted facts put it in (CLIP-36): nothing new is persisted, so a
 *  reload, the list badge and this screen cannot disagree.
 *
 *  - a confirmed or finalized project opens ⑤ 완성;
 *  - an unconfirmed one holding an edit plan (or a render of one) opens ④ 다듬기;
 *  - one holding a storyline and no plan opens ③ 스토리라인;
 *  - one holding observations only opens ② 관찰;
 *  - any other opens ① 생성. */
export function stepForProject(project: ClipProject): ClipStep {
  const state = clipState(project)
  if (state !== 'draft' && state !== 'refining') return BY_STATE[state]
  if (project.editPlanRevision > 0 || project.result) return 'refine'
  if (project.storyline) return 'storyline'
  if (hasObservations(project)) return 'observe'
  return 'generate'
}

/** Whether the project holds stored observations of at least one source (CLIP-234). */
function hasObservations(project: ClipProject): boolean {
  return project.observations?.status === 'available' && project.observations.sources.length > 0
}

/** ①'s first screen: a project holding an instruction, a source or a running job opens on 확인,
 *  any other on 영상 종류 (CLIP-225). */
export function preparationOpening(material: {
  instruction?: string
  sources: number
  running: boolean
}): 'review' | 'kind' {
  return material.instruction?.trim() || material.sources > 0 || material.running
    ? 'review'
    : 'kind'
}
