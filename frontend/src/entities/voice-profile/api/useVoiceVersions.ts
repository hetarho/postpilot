import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import { VoiceService } from '@/shared/api'
import type { VoiceVersion } from '../model/types'
import { toVoiceVersion, voiceVersionsQueryKey } from './voice-queries'

/** One query per screen that shows it. These three lists were one bundled hook, so opening the
 *  profile fetched the version history, the pending confirmations, and the validation records it
 *  no longer renders. */
export function useVoiceVersions(ownerId: string): {
  versions: VoiceVersion[]
  isPending: boolean
} {
  const transport = useTransport()
  const query = useQuery({
    queryKey: voiceVersionsQueryKey(transport, ownerId),
    queryFn: () => createClient(VoiceService, transport).listVoiceProfileVersions({}),
    enabled: ownerId !== '',
  })
  return { versions: query.data?.versions.map(toVoiceVersion) ?? [], isPending: query.isPending }
}
