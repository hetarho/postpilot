import { create } from '@bufbuild/protobuf'
import { ConnectError } from '@connectrpc/connect'
import { useMutation } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import type { ModelRef } from '@/entities/model-catalog'
import { GenerationService, ModelRefSchema } from '@/shared/api'

export function useStartRevision() {
  const queryClient = useQueryClient()
  const mutation = useMutation(GenerationService.method.startRevision, {
    onSuccess: (_response, request) => {
      if (request.saveAsRule) {
        void queryClient.invalidateQueries({ queryKey: ['voice-profile'] })
      }
    },
  })

  return {
    ...mutation,
    errorMessage: mutation.error ? ConnectError.from(mutation.error).rawMessage : '',
    start: (postSlug: string, instruction: string, saveAsRule: boolean, writeModel: ModelRef) =>
      mutation.mutateAsync({
        postSlug,
        instruction,
        saveAsRule,
        writeModel: create(ModelRefSchema, writeModel),
      }),
  }
}
