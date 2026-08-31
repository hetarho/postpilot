import { create } from '@bufbuild/protobuf'
import { useMutation } from '@connectrpc/connect-query'
import type { ModelRef } from '@/entities/model-catalog'
import { appFailureFromConnect, ModelExperimentService, ModelRefSchema } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'

export function useStartWriteExperiment() {
  const mutation = useMutation(ModelExperimentService.method.startWriteExperiment)
  return {
    ...mutation,
    errorMessage: mutation.error ? formatAppFailure(appFailureFromConnect(mutation.error)) : '',
    start: (
      postSlug: string,
      observeModel: ModelRef | undefined,
      modelA: ModelRef,
      modelB: ModelRef,
      targetLength?: number,
    ) =>
      mutation.mutateAsync({
        postSlug,
        observeModel: observeModel ? create(ModelRefSchema, observeModel) : undefined,
        modelA: create(ModelRefSchema, modelA),
        modelB: create(ModelRefSchema, modelB),
        targetLength,
      }),
  }
}
