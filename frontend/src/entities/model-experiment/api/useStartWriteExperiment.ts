import type { MessageShape } from '@bufbuild/protobuf'
import type { ModelRef } from '@/entities/model-catalog/@x/model-experiment'
import type { ModelExperimentService } from '@/shared/api'
import type { ExperimentOriginName } from '../model/types'
import { useRetiredExperimentMutation } from './useRetiredExperimentMutation'

export function useStartWriteExperiment() {
  const { refuse, ...state } = useRetiredExperimentMutation()
  const start: (
    postSlug: string,
    origin: ExperimentOriginName,
    observeModel: ModelRef | undefined,
    modelA: ModelRef,
    modelB: ModelRef,
    targetLength?: number,
    reobserveFiles?: readonly string[],
    extras?: readonly ModelRef[],
  ) => Promise<MessageShape<typeof ModelExperimentService.method.startWriteExperiment.output>> =
    refuse
  return { ...state, start }
}
