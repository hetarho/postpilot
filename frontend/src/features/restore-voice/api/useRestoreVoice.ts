import { Code, ConnectError } from '@connectrpc/connect'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { listPostsQueryKey, postDetailQueriesKey } from '@/entities/post'
import { invalidateVoiceScope, upsertCachedVoice } from '@/entities/voice'
import { VoiceService } from '@/shared/api'

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
  return {
    ...mutation,
    errorMessage: mutation.error ? describe(ConnectError.from(mutation.error).code) : '',
    restore: (voiceId: string) => mutation.mutateAsync({ voiceId }),
  }
}

function describe(code: Code): string {
  switch (code) {
    case Code.AlreadyExists:
      return '같은 이름의 말투가 이미 있어요. 이름을 바꾼 뒤 복원해 주세요.'
    case Code.NotFound:
      return '말투를 찾을 수 없어요. 목록을 새로 고쳐 주세요.'
    default:
      return '말투를 복원하지 못했어요. 다시 시도해 주세요.'
  }
}
