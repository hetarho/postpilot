import { Code, ConnectError } from '@connectrpc/connect'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { listPostsQueryKey, postDetailQueriesKey } from '@/entities/post'
import { invalidateVoiceScope, upsertCachedVoice } from '@/entities/voice'
import { VoiceService } from '@/shared/api'
import { VOICE_NAME_MAX_CHARS } from '@/shared/config'

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
    errorMessage: mutation.error ? describe(ConnectError.from(mutation.error).code) : '',
    rename: (voiceId: string, name: string) => mutation.mutateAsync({ voiceId, name }),
  }
}

function describe(code: Code): string {
  switch (code) {
    case Code.InvalidArgument:
      return `이름은 공백을 빼고 1~${VOICE_NAME_MAX_CHARS}자로 입력해 주세요.`
    case Code.AlreadyExists:
      return '같은 이름의 말투가 이미 있어요.'
    default:
      return '이름을 바꾸지 못했어요. 다시 시도해 주세요.'
  }
}
