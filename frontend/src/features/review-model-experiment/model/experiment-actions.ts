import { needsExperimentReview, type ModelExperiment } from '@/entities/model-experiment'

/** Whether this experiment still offers ANY action.
 *
 *  Every button in the bar is status-gated, so `queued`, `running` and `dismissed` matched none of
 *  them and docked an empty slab: while the run is in flight — exactly the state the user sits and
 *  waits in — a shadowed 32px strip of `surface-highest` covered the last lines of the candidate
 *  text and read as a half-loaded control. The page uses this to decide whether the dock is worth
 *  rendering at all; the component uses it to render nothing when it is not. */
export function hasExperimentActions(experiment: ModelExperiment): boolean {
  return (
    needsExperimentReview(experiment.status) ||
    experiment.status === 'decided' ||
    Boolean(experiment.applyFailure) ||
    Boolean(experiment.adoptionFailure)
  )
}
