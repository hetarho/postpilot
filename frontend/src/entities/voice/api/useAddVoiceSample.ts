import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { VoiceService } from '@/shared/api'
import { connectFailureMessage } from '@/shared/lib'
import { invalidateVoiceMaterials } from './voice-directory-cache'

/** Pastes a post as a 학습 글. It needs no model and enqueues nothing (VOICE-21); the profile's
 *  meter and the directory's count move, so the voice's scope is refetched. */
export function useAddVoiceSample(ownerId: string, voiceId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(VoiceService.method.addVoiceSample, {
    onSuccess: () => invalidateVoiceMaterials(queryClient, transport, ownerId, voiceId),
  })
  return {
    ...mutation,
    errorMessage: connectFailureMessage(mutation.error),
    add: (label: string, body: string) => mutation.mutateAsync({ voiceId, label, body }),
  }
}
