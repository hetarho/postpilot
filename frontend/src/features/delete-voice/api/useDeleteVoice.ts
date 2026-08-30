import { Code, ConnectError } from '@connectrpc/connect'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { listPostsQueryKey, postDetailQueriesKey } from '@/entities/post'
import { invalidateVoiceScope, upsertCachedVoice } from '@/entities/voice'
import { VoiceService } from '@/shared/api'

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
  return {
    ...mutation,
    errorMessage: mutation.error ? describe(ConnectError.from(mutation.error).code) : '',
    remove: (voiceId: string) => mutation.mutateAsync({ voiceId }),
  }
}

function describe(code: Code): string {
  switch (code) {
    case Code.FailedPrecondition:
      return '지금은 삭제할 수 없어요. 기본 말투이거나, 진행 중이거나 아직 결정하지 않은 작업이 있어요.'
    case Code.NotFound:
      return '말투를 찾을 수 없어요. 목록을 새로 고쳐 주세요.'
    default:
      return '말투를 삭제하지 못했어요. 다시 시도해 주세요.'
  }
}
