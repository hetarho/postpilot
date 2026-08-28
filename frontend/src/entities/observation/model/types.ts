import type { Observation } from '@/shared/api'

export function observationByFile(
  observations: readonly Observation[],
): ReadonlyMap<string, Observation> {
  return new Map(observations.map((observation) => [observation.file, observation]))
}
