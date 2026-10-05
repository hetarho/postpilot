import {
  useMutation,
  useQuery,
  useTransport,
  createConnectQueryKey,
} from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { SpeechProfileService, appFailureFromConnect } from '@/shared/api'
import type { AdminSpeechProfile } from '../model/speech'
import { toSpeechAdminBrowse, toSpeechChoice } from './speech-mappers'

export function useSpeechProfiles(qualificationSessionId = '') {
  const query = useQuery(SpeechProfileService.method.listSpeechProfiles, { qualificationSessionId })
  return { ...query, profiles: query.data?.profiles.map(toSpeechChoice) ?? [] }
}

export function useAdminSpeechProfiles() {
  const transport = useTransport()
  const cache = useQueryClient()
  const query = useQuery(SpeechProfileService.method.adminListSpeechProfiles, { refresh: false })
  const refresh = useMutation(SpeechProfileService.method.adminListSpeechProfiles, {
    onSuccess: (data) =>
      cache.setQueryData(
        createConnectQueryKey({
          schema: SpeechProfileService.method.adminListSpeechProfiles,
          input: { refresh: false },
          transport,
          cardinality: 'finite',
        }),
        data,
      ),
  })
  const save = useMutation(SpeechProfileService.method.saveSpeechProfile, {
    onSuccess: () => {
      void cache.invalidateQueries({
        queryKey: createConnectQueryKey({
          schema: SpeechProfileService.method.adminListSpeechProfiles,
          transport,
          cardinality: 'finite',
        }),
      })
      void cache.invalidateQueries({
        queryKey: createConnectQueryKey({
          schema: SpeechProfileService.method.listSpeechProfiles,
          transport,
          cardinality: 'finite',
        }),
      })
    },
  })
  const qualification = useMutation(SpeechProfileService.method.startSpeechQualification)
  return {
    browse: query.data
      ? toSpeechAdminBrowse(query.data)
      : { profiles: [], choices: [], candidates: [], fetchError: '' },
    isPending: query.isPending,
    hasData: query.data !== undefined,
    isError: query.isError || refresh.isError,
    refresh: () => refresh.mutate({ refresh: true }),
    refreshing: refresh.isPending,
    save: (profile: AdminSpeechProfile) =>
      save.mutateAsync({ profile, expectedRevision: profile.revision }),
    saving: save.isPending,
    startQualification: (id: string, revision: bigint, maximumUsd: string) =>
      qualification.mutateAsync({ profileId: id, revision, maximumUsd }),
    qualification: qualification.data
      ? {
          sessionId: qualification.data.sessionId,
          profileId: qualification.data.profileId,
          revision: qualification.data.revision,
          expiresAt: qualification.data.expiresAt,
        }
      : undefined,
    qualifying: qualification.isPending,
    failure:
      save.error || refresh.error || qualification.error || query.error
        ? appFailureFromConnect(save.error ?? refresh.error ?? qualification.error ?? query.error)
        : undefined,
  }
}
