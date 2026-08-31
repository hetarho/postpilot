import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { replaceCachedVoices } from '@/entities/voice'
import { appFailureFromConnect, VoiceService } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'

export function useSetDefaultVoice(ownerId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(VoiceService.method.setDefaultVoice, {
    // The previous default changed too, which is why the server answers with every voice.
    onSuccess: (data) => replaceCachedVoices(queryClient, transport, ownerId, data.voices),
  })
  return {
    ...mutation,
    errorMessage: mutation.error ? formatAppFailure(appFailureFromConnect(mutation.error)) : '',
    setDefault: (voiceId: string) => mutation.mutateAsync({ voiceId }),
  }
}
