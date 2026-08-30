import { useMemo } from 'react'
import { createClient, type Transport } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery, type QueryClient } from '@tanstack/react-query'
import { VoiceService } from '@/shared/api'
import { activeVoices, defaultVoice, deletedVoices, type Voice } from '../model/types'
import { toVoice, voicesQueryKey } from './voice-queries'

/** The one query behind the directory. Shared with the route that resolves the legacy `/voice`
 *  address, so the guard and the screens read and write the same cache entry. */
export function voiceDirectoryQuery(transport: Transport, ownerId: string) {
  return {
    queryKey: voicesQueryKey(transport, ownerId),
    queryFn: () => createClient(VoiceService, transport).listVoices({}),
  }
}

/** Resolves the account's voices outside React — for a `beforeLoad`. A fresh cache entry answers
 *  without a request. A read, never a write: no voice is created here. */
export async function loadVoices(
  queryClient: QueryClient,
  transport: Transport,
  ownerId: string,
): Promise<Voice[]> {
  const response = await queryClient.ensureQueryData(voiceDirectoryQuery(transport, ownerId))
  return response.voices.map(toVoice)
}

export function useVoices(ownerId: string): {
  voices: Voice[]
  active: Voice[]
  deleted: Voice[]
  defaultVoice: Voice | undefined
  isPending: boolean
  isError: boolean
  isFetching: boolean
  refetch: () => void
} {
  const transport = useTransport()
  const query = useQuery({ ...voiceDirectoryQuery(transport, ownerId), enabled: ownerId !== '' })
  const voices = useMemo(() => query.data?.voices.map(toVoice) ?? [], [query.data])
  const active = useMemo(() => activeVoices(voices), [voices])
  const deleted = useMemo(() => deletedVoices(voices), [voices])
  return {
    voices,
    active,
    deleted,
    defaultVoice: defaultVoice(voices),
    isPending: query.isPending,
    isError: query.isError,
    isFetching: query.isFetching,
    refetch: () => void query.refetch(),
  }
}
