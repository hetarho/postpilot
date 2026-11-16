import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { VoiceService } from '@/shared/api'
import { connectFailureMessage } from '@/shared/lib'
import { invalidateVoiceMaterials, withdrawCachedVoiceMaterial } from './voice-directory-cache'

/** Deletes a 학습 글; it enqueues nothing (VOICE-21). */
export function useDeleteVoiceSample(ownerId: string, voiceId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(VoiceService.method.deleteVoiceSample, {
    onSuccess: (_result, input) => {
      if (input.sampleId)
        withdrawCachedVoiceMaterial(queryClient, transport, ownerId, voiceId, input.sampleId)
      invalidateVoiceMaterials(queryClient, transport, ownerId, voiceId)
    },
  })
  return {
    ...mutation,
    errorMessage: connectFailureMessage(mutation.error),
    remove: (sampleId: string) => mutation.mutateAsync({ voiceId, sampleId }),
  }
}
