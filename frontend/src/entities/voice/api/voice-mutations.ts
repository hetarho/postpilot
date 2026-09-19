import { create } from '@bufbuild/protobuf'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { listPostsQueryKey, postDetailQueriesKey } from '@/entities/post/@x/voice'
import {
  appFailureFromConnect,
  contentLanguageToProto,
  ModelRefSchema,
  VoiceService,
  type ContentLanguage,
  type VoiceLayer,
} from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import {
  invalidateVoiceScope,
  replaceCachedVoices,
  upsertCachedVoice,
} from './voice-directory-cache'
import { voiceProfileQueryKey, voicesQueryKey, voiceVersionsQueryKey } from './voice-queries'

/** The voice noun's own writes (ARCH-14's verb line): each is a bare mutation nobody renders, and
 *  the buttons, sheets and confirmations that render around them stay in the verb features. The
 *  proto descriptors and the message schemas stop here (ARCH-17) — a caller hands domain values. */

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
      // A new voice starts empty (spec/legacy/policy/voice.md), so nothing else is stale: only the
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
      invalidateDependentPosts(queryClient, transport)
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

export function useRenameVoice(ownerId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(VoiceService.method.renameVoice, {
    onSuccess: (data) => {
      if (data.voice) upsertCachedVoice(queryClient, transport, ownerId, data.voice)
      // The name is projected onto every post written in the voice and onto its own profile
      // response; none of those rows changed, only what they display.
      if (data.voice) invalidateVoiceScope(queryClient, transport, ownerId, data.voice.id)
      invalidateDependentPosts(queryClient, transport)
    },
  })
  return {
    ...mutation,
    errorMessage: mutation.error ? formatAppFailure(appFailureFromConnect(mutation.error)) : '',
    rename: (voiceId: string, name: string) => mutation.mutateAsync({ voiceId, name }),
  }
}

export function useRestoreVoice(ownerId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(VoiceService.method.restoreVoice, {
    onSuccess: (data) => {
      if (data.voice) upsertCachedVoice(queryClient, transport, ownerId, data.voice)
      // Every post written in the voice loses its tombstone, and its AI controls come back.
      if (data.voice) invalidateVoiceScope(queryClient, transport, ownerId, data.voice.id)
      invalidateDependentPosts(queryClient, transport)
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

export function useSetDefaultVoice(ownerId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(VoiceService.method.setDefaultVoice, {
    // The previous default changed too, which is why the server answers with every voice.
    onSuccess: (data) => replaceCachedVoices(queryClient, transport, ownerId, data.voices),
  })
  return {
    ...mutation,
    errorMessage: mutation.error ? formatAppFailure(appFailureFromConnect(mutation.error)) : '',
    setDefault: (voiceId: string) => mutation.mutateAsync({ voiceId }),
  }
}

/** One overridable profile field. Each caller owns its own mutation so the fields stay
 *  independent: a save in one must not put the others into a pending state. An override publishes
 *  a new whole-profile version, so the version list is stale alongside the profile. */
export function useUpdateVoiceOverride(ownerId: string, voiceId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(VoiceService.method.updateVoiceOverride)
  const refresh = async () => {
    await queryClient.invalidateQueries({
      queryKey: voiceProfileQueryKey(transport, ownerId, voiceId),
    })
    await queryClient.invalidateQueries({
      queryKey: voiceVersionsQueryKey(transport, ownerId, voiceId),
    })
  }
  return {
    ...mutation,
    errorMessage: mutation.error ? formatAppFailure(appFailureFromConnect(mutation.error)) : '',
    override: async (layer: VoiceLayer, field: string, value?: string) => {
      await mutation.mutateAsync({ voiceId, layer, field, value })
      await refresh()
    },
  }
}

/** Adopting an older version publishes a NEW head and destroys no history. */
export function useRestoreVoiceProfile(ownerId: string, voiceId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(VoiceService.method.restoreVoiceProfile, {
    onSuccess: () => {
      for (const queryKey of [
        voiceProfileQueryKey(transport, ownerId, voiceId),
        voiceVersionsQueryKey(transport, ownerId, voiceId),
      ]) {
        void queryClient.invalidateQueries({ queryKey })
      }
    },
  })
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    adopt: (version: bigint) => mutation.mutateAsync({ voiceId, version }),
  }
}

function invalidateDependentPosts(
  queryClient: ReturnType<typeof useQueryClient>,
  transport: ReturnType<typeof useTransport>,
): void {
  void queryClient.invalidateQueries({ queryKey: listPostsQueryKey(transport) })
  void queryClient.invalidateQueries({ queryKey: postDetailQueriesKey(transport) })
}
