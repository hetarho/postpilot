import { create } from '@bufbuild/protobuf'
import type { Transport } from '@connectrpc/connect'
import type { QueryClient } from '@tanstack/react-query'
import { ListVoicesResponseSchema, type ListVoicesResponse, type ProtoVoice } from '@/shared/api'
import { sortVoices } from '../model/types'
import {
  voiceConfirmationsQueryKey,
  voiceProfileQueryKey,
  voiceValidationsQueryKey,
  voiceVersionsQueryKey,
  voicesQueryKey,
} from './voice-queries'

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
  for (const queryKey of [
    voiceProfileQueryKey(transport, ownerId, voiceId),
    voiceVersionsQueryKey(transport, ownerId, voiceId),
    voiceConfirmationsQueryKey(transport, ownerId, voiceId),
    voiceValidationsQueryKey(transport, ownerId, voiceId),
  ]) {
    void queryClient.invalidateQueries({ queryKey })
  }
}
