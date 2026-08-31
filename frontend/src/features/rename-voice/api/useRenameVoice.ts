import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { listPostsQueryKey, postDetailQueriesKey } from '@/entities/post'
import { invalidateVoiceScope, upsertCachedVoice } from '@/entities/voice'
import { appFailureFromConnect, VoiceService } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'

export function useRenameVoice(ownerId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(VoiceService.method.renameVoice, {
    onSuccess: (data) => {
      if (data.voice) upsertCachedVoice(queryClient, transport, ownerId, data.voice)
      // The name is projected onto every post written in the voice and onto its own profile
      // response; none of those rows changed, only what they display.
      if (data.voice) invalidateVoiceScope(queryClient, transport, ownerId, data.voice.id)
      void queryClient.invalidateQueries({ queryKey: listPostsQueryKey(transport) })
      void queryClient.invalidateQueries({ queryKey: postDetailQueriesKey(transport) })
    },
  })
  return {
    ...mutation,
    errorMessage: mutation.error ? formatAppFailure(appFailureFromConnect(mutation.error)) : '',
    rename: (voiceId: string, name: string) => mutation.mutateAsync({ voiceId, name }),
  }
}
