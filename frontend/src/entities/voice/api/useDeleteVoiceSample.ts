import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { appFailureFromConnect, VoiceService } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import { voiceProfileQueryKey } from './voice-queries'

export function useDeleteVoiceSample(ownerId: string, voiceId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(VoiceService.method.deleteVoiceSample, {
    onSuccess: () =>
      queryClient.invalidateQueries({
        queryKey: voiceProfileQueryKey(transport, ownerId, voiceId),
      }),
  })
  return {
    ...mutation,
    errorMessage: mutation.error ? formatAppFailure(appFailureFromConnect(mutation.error)) : '',
    remove: (sampleId: string) => mutation.mutateAsync({ voiceId, sampleId }),
  }
}
