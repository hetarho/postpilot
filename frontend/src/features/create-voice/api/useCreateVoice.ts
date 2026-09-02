import { create } from '@bufbuild/protobuf'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { upsertCachedVoice, voicesQueryKey } from '@/entities/voice'
import {
  appFailureFromConnect,
  contentLanguageToProto,
  ModelRefSchema,
  type ContentLanguage,
  VoiceService,
} from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'

export interface CreateVoiceInput {
  name: string
  sourceLanguage: ContentLanguage
  /** The optional 말투 설명. Empty means the plain creation: no model, no job. */
  description: string
  /** The account's analyze-stage choice, required only alongside a description. */
  analyzeModel: { providerId: string; modelId: string } | null
}

export function useCreateVoice(ownerId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(VoiceService.method.createVoice, {
    onSuccess: (data) => {
      // A new voice starts empty (spec/policy/voice.md), so nothing else is stale: only the
      // directory gains a row. A seeded one is still empty right now — its profile is written
      // by the job the response names, and the voice screen reads that profile itself.
      if (data.voice) upsertCachedVoice(queryClient, transport, ownerId, data.voice)
    },
    onError: () => {
      // Creation and seeding are separable outcomes: the server may refuse to START the seed
      // (a quota, a provider) after the voice row already exists. Refetching the directory is
      // what keeps that voice from looking invisible while its name is genuinely taken.
      void queryClient.invalidateQueries({ queryKey: voicesQueryKey(transport, ownerId) })
    },
  })
  return {
    ...mutation,
    errorMessage: mutation.error ? formatAppFailure(appFailureFromConnect(mutation.error)) : '',
    create: ({ name, sourceLanguage, description, analyzeModel }: CreateVoiceInput) =>
      mutation.mutateAsync({
        name,
        sourceLanguage: contentLanguageToProto(sourceLanguage),
        description,
        analyzeModel: analyzeModel ? create(ModelRefSchema, analyzeModel) : undefined,
      }),
  }
}
