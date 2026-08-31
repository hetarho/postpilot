import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { listPostsQueryKey, postDetailQueriesKey } from '@/entities/post'
import { invalidateVoiceScope, upsertCachedVoice } from '@/entities/voice'
import { appFailureFromConnect, VoiceService } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'

export function useDeleteVoice(ownerId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(VoiceService.method.deleteVoice, {
    onSuccess: (data) => {
      // A soft delete: the server returns the same voice as a tombstone, and it stays in the
      // directory (in the deleted section). Nothing is removed from any cache — posts and
      // history are intact and only display differently.
      if (data.voice) upsertCachedVoice(queryClient, transport, ownerId, data.voice)
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
    remove: (voiceId: string) => mutation.mutateAsync({ voiceId }),
  }
}
