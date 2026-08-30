import { useMemo } from 'react'
import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import { VoiceService } from '@/shared/api'
import type { VoiceProfile } from '../model/types'
import { toVoiceProfile, voiceProfileQueryKey } from './voice-queries'

export function useVoiceProfile(
  ownerId: string,
  voiceId: string,
): {
  profile: VoiceProfile | undefined
  isPending: boolean
  isError: boolean
  refetch: () => void
} {
  const transport = useTransport()
  const query = useQuery({
    queryKey: voiceProfileQueryKey(transport, ownerId, voiceId),
    queryFn: () => createClient(VoiceService, transport).getVoiceProfile({ voiceId }),
    enabled: ownerId !== '' && voiceId !== '',
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
