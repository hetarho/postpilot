import type { MessageShape } from '@bufbuild/protobuf'
import type { ModelRef } from '@/entities/model-catalog/@x/model-experiment'
import type { ModelExperimentService } from '@/shared/api'
import { useRetiredExperimentMutation } from './useRetiredExperimentMutation'

export function useStartVoiceReflection() {
  const { refuse, ...state } = useRetiredExperimentMutation()
  const start: (
    voiceId: string,
    promptKey: string,
    modelA: ModelRef,
    modelB: ModelRef,
    extras?: readonly ModelRef[],
  ) => Promise<
    MessageShape<typeof ModelExperimentService.method.startVoiceReflectionExperiment.output>
  > = refuse
  return { ...state, start }
}
