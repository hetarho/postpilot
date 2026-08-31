import { create } from '@bufbuild/protobuf'
import { useMutation } from '@connectrpc/connect-query'
import type { ModelRef } from '@/entities/model-catalog'
import { appFailureFromConnect, GenerationService, ModelRefSchema } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'

export function useStartGeneration() {
  const mutation = useMutation(GenerationService.method.startGeneration)

  return {
    ...mutation,
    errorMessage: mutation.error ? formatAppFailure(appFailureFromConnect(mutation.error)) : '',
    start: (
      postSlug: string,
      observeModel: ModelRef | undefined,
      writeModel: ModelRef,
      targetLength?: number,
    ) =>
      mutation.mutateAsync({
        postSlug,
        observeModel: observeModel ? create(ModelRefSchema, observeModel) : undefined,
        writeModel: create(ModelRefSchema, writeModel),
        targetLength,
      }),
  }
}
