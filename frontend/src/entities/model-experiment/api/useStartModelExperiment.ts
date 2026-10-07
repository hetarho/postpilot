import type { MessageShape } from '@bufbuild/protobuf'
import type { ModelRef } from '@/entities/model-catalog/@x/model-experiment'
import type { ModelExperimentService } from '@/shared/api'
import { useRetiredExperimentMutation } from './useRetiredExperimentMutation'

export function useStartModelExperiment() {
  const { refuse, ...state } = useRetiredExperimentMutation()
  const startObserve: (
    postSlug: string,
    modelA: ModelRef,
    modelB: ModelRef,
    extras?: readonly ModelRef[],
  ) => Promise<MessageShape<typeof ModelExperimentService.method.startObserveExperiment.output>> =
    refuse
  return { ...state, startObserve }
}
