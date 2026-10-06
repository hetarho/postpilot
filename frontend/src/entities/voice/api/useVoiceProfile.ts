import { useCallback, useMemo } from 'react'
import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import { VoiceService } from '@/shared/api'
import type { VoiceProfile } from '../model/types'
import { toVoiceProfile, voiceAnalysisQueryKey } from './voice-queries'

export function useVoiceProfile(
  ownerId: string,
  voiceId: string,
): {
  profile: VoiceProfile | undefined
  isPending: boolean
  isError: boolean
  refetch: () => void
  refresh: () => Promise<VoiceProfile>
} {
  const transport = useTransport()
  const query = useQuery({
    queryKey: voiceAnalysisQueryKey(transport, ownerId, voiceId),
    queryFn: () => createClient(VoiceService, transport).getVoiceProfile({ voiceId }),
    enabled: ownerId !== '' && voiceId !== '',
  })
  const profile = useMemo(
    () => (query.data ? toVoiceProfile(query.data.profile) : undefined),
    [query.data],
  )
  const refreshQuery = query.refetch
  const refresh = useCallback(async () => {
    if (!ownerId || !voiceId) throw new Error('An owned writing voice is required')
    const result = await refreshQuery({ throwOnError: true })
    const current = result.data?.profile
    if (!current?.voice || current.voice.id !== voiceId)
      throw new Error('The refreshed writing voice was not confirmed')
    return toVoiceProfile(current)
  }, [ownerId, voiceId, refreshQuery])
  return {
    profile,
    isPending: query.isPending,
    isError: query.isError,
    refetch: () => void query.refetch(),
    refresh,
  }
}
