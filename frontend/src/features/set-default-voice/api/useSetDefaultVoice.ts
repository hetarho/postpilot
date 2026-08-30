import { Code, ConnectError } from '@connectrpc/connect'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { replaceCachedVoices } from '@/entities/voice'
import { VoiceService } from '@/shared/api'

export function useSetDefaultVoice(ownerId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(VoiceService.method.setDefaultVoice, {
    // The previous default changed too, which is why the server answers with every voice.
    onSuccess: (data) => replaceCachedVoices(queryClient, transport, ownerId, data.voices),
  })
  return {
    ...mutation,
    errorMessage: mutation.error ? describe(ConnectError.from(mutation.error).code) : '',
    setDefault: (voiceId: string) => mutation.mutateAsync({ voiceId }),
  }
}

function describe(code: Code): string {
  switch (code) {
    case Code.FailedPrecondition:
      return '삭제된 말투는 기본으로 설정할 수 없어요. 먼저 복원해 주세요.'
    case Code.NotFound:
      return '말투를 찾을 수 없어요. 목록을 새로 고쳐 주세요.'
    default:
      return '기본 말투를 바꾸지 못했어요. 다시 시도해 주세요.'
  }
}
