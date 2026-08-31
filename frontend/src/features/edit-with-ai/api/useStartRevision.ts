import { create } from '@bufbuild/protobuf'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import type { ModelRef } from '@/entities/model-catalog'
import { voiceProfileQueryKey } from '@/entities/voice'
import { appFailureFromConnect, GenerationService, ModelRefSchema } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'

export function useStartRevision(ownerId: string, voiceId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(GenerationService.method.startRevision, {
    onSuccess: (_response, request) => {
      if (request.saveAsRule) {
        // The rule belongs to the post's frozen/current voice. Invalidating the broad
        // `voice-profile` prefix would make unrelated voices refetch and breaks the
        // account+voice cache boundary that keeps contradictory profiles independent.
        void queryClient.invalidateQueries({
          queryKey: voiceProfileQueryKey(transport, ownerId, voiceId),
        })
      }
    },
  })

  return {
    ...mutation,
    errorMessage: mutation.error ? formatAppFailure(appFailureFromConnect(mutation.error)) : '',
    start: (postSlug: string, instruction: string, saveAsRule: boolean, writeModel: ModelRef) =>
      mutation.mutateAsync({
        postSlug,
        instruction,
        saveAsRule,
        writeModel: create(ModelRefSchema, writeModel),
      }),
  }
}
