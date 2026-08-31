import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { upsertCachedVoice } from '@/entities/voice'
import {
  appFailureFromConnect,
  contentLanguageToProto,
  type ContentLanguage,
  VoiceService,
} from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'

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
    errorMessage: mutation.error ? formatAppFailure(appFailureFromConnect(mutation.error)) : '',
    create: (name: string, sourceLanguage: ContentLanguage) =>
      mutation.mutateAsync({
        name,
        sourceLanguage: contentLanguageToProto(sourceLanguage),
      }),
  }
}
