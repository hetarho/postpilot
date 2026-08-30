import { clone, create } from '@bufbuild/protobuf'
import { ConnectError } from '@connectrpc/connect'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useIsMutating, useQueryClient } from '@tanstack/react-query'
import {
  type GetVoiceProfileResponse,
  GetVoiceProfileResponseSchema,
  VoiceService,
} from '@/shared/api'
import { voiceProfileQueryKey } from './voice-queries'

export function useUpdateVoiceProfile(ownerId: string, voiceId: string) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutationKey = [...VOICE_PROFILE_UPDATE_KEY, ownerId, voiceId] as const
  const isSaving = useIsMutating({ mutationKey }) > 0
  const mutation = useMutation(VoiceService.method.updateVoiceProfile, {
    mutationKey,
    onSuccess: (data, saved) => {
      queryClient.setQueryData<GetVoiceProfileResponse>(
        voiceProfileQueryKey(transport, ownerId, voiceId),
        (current) => {
          if (!current?.profile || !data.profile) {
            return create(GetVoiceProfileResponseSchema, { profile: data.profile })
          }
          // The response is a server snapshot from some point during the request. An
          // analysis/profile refetch may have installed newer companion fields before
          // it arrived, so mirror only the field this mutation actually settled.
          const next = clone(GetVoiceProfileResponseSchema, current)
          if (saved.styleguide !== undefined) next.profile!.styleguide = data.profile.styleguide
          if (saved.rules !== undefined) next.profile!.rules = data.profile.rules
          next.profile!.updatedAt = data.profile.updatedAt
          return next
        },
      )
    },
  })
  return {
    ...mutation,
    isSaving,
    errorMessage: mutation.error ? ConnectError.from(mutation.error).rawMessage : '',
    saveStyleguide: (styleguide: string) => mutation.mutateAsync({ voiceId, styleguide }),
    saveRules: (rules: string) => mutation.mutateAsync({ voiceId, rules }),
  }
}

const VOICE_PROFILE_UPDATE_KEY = ['voice-profile', 'update'] as const
