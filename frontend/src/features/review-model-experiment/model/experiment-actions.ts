import type { ModelExperiment } from '@/entities/model-experiment'

/** Paid legacy records offer reading and copying, never a new legacy mutation. */
export function hasExperimentActions(experiment: ModelExperiment): boolean {
  void experiment
  return false
}
