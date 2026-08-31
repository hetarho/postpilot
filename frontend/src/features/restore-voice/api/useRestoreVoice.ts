import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { listPostsQueryKey, postDetailQueriesKey } from '@/entities/post'
import { invalidateVoiceScope, upsertCachedVoice } from '@/entities/voice'
import { appFailureFromConnect, VoiceService } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'

export function useRestoreVoice(ownerId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(VoiceService.method.restoreVoice, {
    onSuccess: (data) => {
      if (data.voice) upsertCachedVoice(queryClient, transport, ownerId, data.voice)
      // Every post written in the voice loses its tombstone, and its AI controls come back.
      if (data.voice) invalidateVoiceScope(queryClient, transport, ownerId, data.voice.id)
      void queryClient.invalidateQueries({ queryKey: listPostsQueryKey(transport) })
      void queryClient.invalidateQueries({ queryKey: postDetailQueriesKey(transport) })
    },
  })
  const failure = mutation.error ? appFailureFromConnect(mutation.error) : undefined
  return {
    ...mutation,
    failure,
    errorMessage: failure ? formatAppFailure(failure) : '',
    restore: (voiceId: string) => mutation.mutateAsync({ voiceId }),
  }
}
