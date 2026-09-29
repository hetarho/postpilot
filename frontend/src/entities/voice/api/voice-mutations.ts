import { create } from '@bufbuild/protobuf'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { invalidatePostsDependingOn } from '@/entities/post/@x/voice'
import { appFailureFromConnect, GetVoiceProfileResponseSchema, VoiceService } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import {
  invalidateVoiceScope,
  replaceCachedVoices,
  upsertCachedVoice,
} from './voice-directory-cache'
import { voiceAnalysisQueryKey, voicesQueryKey } from './voice-queries'

/** The voice noun's own writes (ARCH-14's verb line): each is a bare mutation nobody renders, and
 *  the buttons, sheets and confirmations that render around them stay in the verb features. The
 *  proto descriptors and the message schemas stop here (ARCH-17) — a caller hands domain values. */

/** A voice is created by name alone, as a Korean voice not yet made (VOICE-10). */
export interface CreateVoiceInput {
  name: string
}

export function useCreateVoice(ownerId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(VoiceService.method.createVoice, {
    onSuccess: (data) => {
      // A new voice starts empty (VOICE-10), so nothing else is stale: only the
      // directory gains a row.
      if (data.voice) upsertCachedVoice(queryClient, transport, ownerId, data.voice)
    },
  })
  return {
    ...mutation,
    errorMessage: mutation.error ? formatAppFailure(appFailureFromConnect(mutation.error)) : '',
    create: ({ name }: CreateVoiceInput) => mutation.mutateAsync({ name }),
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
      invalidatePostsDependingOn(queryClient, transport, { voiceId: data.voice?.id })
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
      invalidatePostsDependingOn(queryClient, transport, { voiceId: data.voice?.id })
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
      invalidatePostsDependingOn(queryClient, transport, { voiceId: data.voice?.id })
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
    /** Leaves the account with no 기본 (VOICE-12): an empty id is the clear. */
    clearDefault: () => mutation.mutateAsync({ voiceId: '' }),
  }
}

/** 이전 분석으로 되돌리기 (VOICE-30): the previous analysis becomes current, the replaced one is
 *  discarded, and the directory's analysis date moves with it. */
export function useRestorePreviousVoiceAnalysis(ownerId: string, voiceId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(VoiceService.method.restorePreviousVoiceAnalysis, {
    onSuccess: (data) => {
      queryClient.setQueryData(
        voiceAnalysisQueryKey(transport, ownerId, voiceId),
        create(GetVoiceProfileResponseSchema, { profile: data.profile }),
      )
      void queryClient.invalidateQueries({ queryKey: voicesQueryKey(transport, ownerId) })
    },
  })
  const failure = mutation.error ? appFailureFromConnect(mutation.error) : undefined
  return {
    ...mutation,
    failure,
    errorMessage: failure ? formatAppFailure(failure) : '',
    restore: () => mutation.mutateAsync({ voiceId }),
  }
}
