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
  finished: 'finish',
}

/** An unreached result tab never implies confirmation. */
export function stepForProject(project: ClipProject): ClipStep {
  return BY_STATE[clipState(project)]
}
