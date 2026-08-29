import { create } from '@bufbuild/protobuf'
import { ConnectError } from '@connectrpc/connect'
import { useMutation } from '@connectrpc/connect-query'
import type { ModelRef } from '@/entities/model-catalog'
import { ModelExperimentService, ModelRefSchema } from '@/shared/api'

export function useStartWriteExperiment() {
  const mutation = useMutation(ModelExperimentService.method.startWriteExperiment)
  return {
    ...mutation,
    errorMessage: mutation.error ? ConnectError.from(mutation.error).rawMessage : '',
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
