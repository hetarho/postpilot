import {
  requiredClipSources,
  type ClipEditPlan,
  type ClipEditingState,
  type RetainedClipSource,
} from '@/entities/clip-plan'
import type { ClipStep } from './steps'

/** Which originals the session must hold for the step it is on (CLIP-23). ① needs every source,
 *  because a fresh AI run reads them all; ② needs only the sources its cuts play. Narrowing the
 *  set is what RELEASES the picked originals, so this is the rule, not a filter. */
export function requiredSourcesForStep(
  step: ClipStep,
  draft: ClipEditPlan,
  plan: ClipEditingState | undefined,
): readonly RetainedClipSource[] | undefined {
  return step === 'refine' && plan ? requiredClipSources(draft, plan.sources) : undefined
}

/** What the retry beside a failed sound switch does. A conflict means the server moved on, so
 *  the setting is re-applied to the plan it moved to; anything else is simply sent again —
 *  retrying a conflict as a plain save would only be refused a second time (CLIP-39). */
export function soundRetryAction(correction: {
  failure?: { reason: string }
  reapply: () => void
  save: () => Promise<unknown>
}) {
  return () => {
    if (correction.failure?.reason === 'CLIP_PLAN_CONFLICT') correction.reapply()
    else void correction.save()
  }
}

/** Whether the step the owner is looking at may still be left without losing work. */
export function unsavedCorrection(dirty: boolean, finalized: unknown) {
  return dirty && !finalized
}
