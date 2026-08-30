import { Code, ConnectError } from '@connectrpc/connect'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { upsertCachedVoice } from '@/entities/voice'
import { VoiceService } from '@/shared/api'
import { VOICE_NAME_MAX_CHARS } from '@/shared/config'

export function useCreateVoice(ownerId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(VoiceService.method.createVoice, {
    onSuccess: (data) => {
      // A new voice starts empty (spec/policy/voice.md), so nothing else is stale: only the
      // directory gains a row.
      if (data.voice) upsertCachedVoice(queryClient, transport, ownerId, data.voice)
    },
  })
  return {
    ...mutation,
    errorMessage: mutation.error ? describe(ConnectError.from(mutation.error).code) : '',
    create: (name: string) => mutation.mutateAsync({ name }),
  }
}

function describe(code: Code): string {
  switch (code) {
    case Code.InvalidArgument:
      return `이름은 공백을 빼고 1~${VOICE_NAME_MAX_CHARS}자로 입력해 주세요.`
    case Code.AlreadyExists:
      return '같은 이름의 말투가 이미 있어요.'
    default:
      return '말투를 만들지 못했어요. 다시 시도해 주세요.'
  }
}
