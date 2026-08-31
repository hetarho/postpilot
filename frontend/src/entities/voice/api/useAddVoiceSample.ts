import { create } from '@bufbuild/protobuf'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { appFailureFromConnect, ModelRefSchema, VoiceService } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import { voiceProfileQueryKey } from './voice-queries'

export function useAddVoiceSample(ownerId: string, voiceId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(VoiceService.method.addVoiceSample, {
    onSuccess: () =>
      queryClient.invalidateQueries({
        queryKey: voiceProfileQueryKey(transport, ownerId, voiceId),
      }),
  })
  return {
    ...mutation,
    errorMessage: mutation.error ? formatAppFailure(appFailureFromConnect(mutation.error)) : '',
    add: (label: string, body: string, model: { providerId: string; modelId: string }) =>
      mutation.mutateAsync({ voiceId, label, body, model: create(ModelRefSchema, model) }),
  }
}
