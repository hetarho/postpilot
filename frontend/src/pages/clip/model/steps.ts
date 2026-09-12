import i18next from 'i18next'
import { clipState, type ClipProject, type ClipState } from '@/entities/clip-project'

export type ClipStep = 'generate' | 'refine' | 'finish'

/** The project's lifecycle as three steps (CLIP-36, THEME-39). One list, so the bar and the
 *  panels cannot drift. */
export function clipSteps(): ReadonlyArray<{ value: ClipStep; label: string }> {
  return [
    { value: 'generate', label: i18next.t('steps.generate', { ns: 'clips' }) },
    { value: 'refine', label: i18next.t('steps.refine', { ns: 'clips' }) },
    { value: 'finish', label: i18next.t('steps.finish', { ns: 'clips' }) },
  ]
}

export function clipStepLabel(step: ClipStep): string {
  return clipSteps().find((item) => item.value === step)?.label ?? ''
}

const BY_STATE: Record<ClipState, ClipStep> = {
  draft: 'generate',
  refining: 'refine',
  finished: 'refine',
}

/** Rendering leaves the owner in correction with preview and download available.
 *  Selecting another step changes nothing on the server.
 *
 *  A FAILED attempt is the one case where the state is not the answer: CLIP-26 owes the owner an
 *  explicit retry, and the retry lives on the step that started the work — a generation is
 *  re-approved on ①, a rerender is re-run from ②. The previous result remains available. */
export function stepForProject(project: ClipProject): ClipStep {
  if (project.latestJob?.status === 'failed')
    return project.latestJob.kind === 'render_clip' ? 'refine' : 'generate'
  return BY_STATE[clipState(project)]
}
