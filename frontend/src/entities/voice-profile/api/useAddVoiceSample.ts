import { create } from '@bufbuild/protobuf'
import { ConnectError } from '@connectrpc/connect'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { ModelRefSchema, VoiceService } from '@/shared/api'
import { voiceProfileQueryKey } from './voice-queries'

export function useAddVoiceSample(ownerId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(VoiceService.method.addVoiceSample, {
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: voiceProfileQueryKey(transport, ownerId) }),
  })
  return {
    ...mutation,
    errorMessage: mutation.error ? ConnectError.from(mutation.error).rawMessage : '',
    add: (label: string, body: string, model: { providerId: string; modelId: string }) =>
      mutation.mutateAsync({ label, body, model: create(ModelRefSchema, model) }),
  }
}
