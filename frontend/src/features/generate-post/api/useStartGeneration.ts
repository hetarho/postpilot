import { create } from '@bufbuild/protobuf'
import { ConnectError } from '@connectrpc/connect'
import { useMutation } from '@connectrpc/connect-query'
import type { ModelRef } from '@/entities/model-catalog'
import { GenerationService, ModelRefSchema } from '@/shared/api'

export function useStartGeneration() {
  const mutation = useMutation(GenerationService.method.startGeneration)

  return {
    ...mutation,
    errorMessage: mutation.error ? ConnectError.from(mutation.error).rawMessage : '',
    start: (postSlug: string, observeModel: ModelRef | undefined, writeModel: ModelRef) =>
      mutation.mutateAsync({
        postSlug,
        observeModel: observeModel ? create(ModelRefSchema, observeModel) : undefined,
        writeModel: create(ModelRefSchema, writeModel),
      }),
  }
}
