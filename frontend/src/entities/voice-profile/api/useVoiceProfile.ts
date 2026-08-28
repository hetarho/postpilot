import { useMemo } from 'react'
import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import { VoiceService } from '@/shared/api'
import type { VoiceProfile } from '../model/types'
import { toVoiceProfile, voiceProfileQueryKey } from './voice-queries'

export function useVoiceProfile(ownerId: string): {
  profile: VoiceProfile | undefined
  isPending: boolean
  isError: boolean
  refetch: () => void
} {
  const transport = useTransport()
  const query = useQuery({
    queryKey: voiceProfileQueryKey(transport, ownerId),
    queryFn: () => createClient(VoiceService, transport).getVoiceProfile({}),
    enabled: ownerId !== '',
  })
  const profile = useMemo(
    () => (query.data ? toVoiceProfile(query.data.profile) : undefined),
    [query.data],
  )
  return {
    profile,
    isPending: query.isPending,
    isError: query.isError,
    refetch: () => void query.refetch(),
  }
}
