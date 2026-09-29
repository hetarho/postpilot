import { create } from '@bufbuild/protobuf'
import { useMutation } from '@connectrpc/connect-query'
import type { ModelRef } from '@/entities/model-catalog/@x/model-experiment'
import { appFailureFromConnect, ModelExperimentService, ModelRefSchema } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'

/** 말투 반영 비교's start (MODEL-67): the write pair writes one answered prompt of one voice. */
export function useStartVoiceReflection() {
  const mutation = useMutation(ModelExperimentService.method.startVoiceReflectionExperiment)
  const failure = mutation.error ? appFailureFromConnect(mutation.error) : undefined
  return {
    isPending: mutation.isPending,
    isError: mutation.isError,
    failure,
    errorMessage: failure ? formatAppFailure(failure) : '',
    start: (voiceId: string, promptKey: string, modelA: ModelRef, modelB: ModelRef) =>
      mutation.mutateAsync({
        voiceId,
        promptKey,
        modelA: create(ModelRefSchema, modelA),
        modelB: create(ModelRefSchema, modelB),
      }),
  }
}
