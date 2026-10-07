import { clone, create } from '@bufbuild/protobuf'
import type { Transport } from '@connectrpc/connect'
import type { QueryClient } from '@tanstack/react-query'
import {
  ListVoicesResponseSchema,
  VoiceProfileSchema,
  VoiceNoticeSchema,
  VoiceNoticeKind,
  type ListVoicesResponse,
  type ProtoVoice,
  type ProtoVoiceProfile,
} from '@/shared/api'
import { sortVoices } from '../model/types'
import { voiceAnalysisQueryKey, voiceSampleQueryKey, voicesQueryKey } from './voice-queries'

/** Installs a whole directory the server just returned — SetDefaultVoice answers with every voice,
 *  because the previous default changed too. */
export function replaceCachedVoices(
  queryClient: QueryClient,
  transport: Transport,
  ownerId: string,
  voices: ProtoVoice[],
): void {
  queryClient.setQueryData(
    voicesQueryKey(transport, ownerId),
    create(ListVoicesResponseSchema, { voices }),
  )
}

/** Merges one voice the server returned into the cached directory. With nothing cached there is
 *  nothing to merge into — one voice is not a directory — so the entry is marked stale instead of
 *  being seeded with a list that would hide every other voice. */
export function upsertCachedVoice(
  queryClient: QueryClient,
  transport: Transport,
  ownerId: string,
  voice: ProtoVoice,
): void {
  const key = voicesQueryKey(transport, ownerId)
  const current = queryClient.getQueryData<ListVoicesResponse>(key)
  if (!current) {
    void queryClient.invalidateQueries({ queryKey: key })
    return
  }
  const others = current.voices.filter((cached) => cached.id !== voice.id)
  queryClient.setQueryData(
    key,
    create(ListVoicesResponseSchema, { voices: sortVoices([...others, voice]) }),
  )
}

/** Marks everything read under one voice stale. The profile response carries the voice summary,
 *  so a rename, delete or restore leaves it wrong until it is re-read. */
export function invalidateVoiceScope(
  queryClient: QueryClient,
  transport: Transport,
  ownerId: string,
  voiceId: string,
): void {
  void queryClient.invalidateQueries({
    queryKey: voiceAnalysisQueryKey(transport, ownerId, voiceId),
  })
}

/** A 학습 글 was added, answered or deleted: the profile's list and meter moved, and so did the
 *  directory row's count and share (VOICE-52). */
export function invalidateVoiceMaterials(
  queryClient: QueryClient,
  transport: Transport,
  ownerId: string,
  voiceId: string,
): void {
  for (const queryKey of [
    voiceAnalysisQueryKey(transport, ownerId, voiceId),
    voicesQueryKey(transport, ownerId),
    voiceSampleQueryKey(transport, ownerId, voiceId, '').slice(0, 4),
  ]) {
    void queryClient.invalidateQueries({ queryKey })
  }
}

/** A successful deletion withdraws known examples from the visible accepted profile immediately. */
export function withdrawCachedVoiceMaterial(
  queryClient: QueryClient,
  transport: Transport,
  ownerId: string,
  voiceId: string,
  sampleId: string,
): void {
  const key = voiceAnalysisQueryKey(transport, ownerId, voiceId)
  const current = queryClient.getQueryData<{ profile?: ProtoVoiceProfile }>(key)
  if (current?.profile) {
    const profile = clone(VoiceProfileSchema, current.profile)
    profile.samples = profile.samples.filter((sample) => sample.id !== sampleId)
    if (profile.voice) profile.voice.materialCount = profile.samples.length
    if (profile.made) profile.notice = create(VoiceNoticeSchema, { kind: VoiceNoticeKind.CHANGED })
    if (profile.analysis) {
      if (profile.analysis.ai)
        profile.analysis.ai.examples = profile.analysis.ai.examples.filter(
          (example) => example.materialId !== sampleId,
        )
      const counted = profile.analysis.counted
      if (counted)
        for (const item of [
          'endings',
          'marks',
          'emoji',
          'shape',
          'openings',
          'adverbs',
          'person',
          'headings',
        ] as const) {
          const value = counted[item]
          if (value?.example?.materialId === sampleId) value.example = undefined
        }
    }
    queryClient.setQueryData(key, { ...current, profile })
  }
  queryClient.removeQueries({
    queryKey: voiceSampleQueryKey(transport, ownerId, voiceId, sampleId),
    exact: true,
  })
}
