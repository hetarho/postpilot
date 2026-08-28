import { ConnectError } from '@connectrpc/connect'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { VoiceService } from '@/shared/api'
import { voiceProfileQueryKey } from './voice-queries'

export function useDeleteVoiceSample(ownerId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(VoiceService.method.deleteVoiceSample, {
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: voiceProfileQueryKey(transport, ownerId) }),
  })
  return {
    ...mutation,
    errorMessage: mutation.error ? ConnectError.from(mutation.error).rawMessage : '',
    remove: (sampleId: string) => mutation.mutateAsync({ sampleId }),
  }
}
