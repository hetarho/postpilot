import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import { VoiceService } from '@/shared/api'
import type { VoiceVersionSample } from '../model/types'
import { voiceVersionSampleQueryKey } from './voice-queries'

/** One version's generation snapshot, fetched only when that version is OPENED.
 *
 *  It is deliberately not part of the version list: a list carrying every post body a voice ever
 *  produced would grow without bound for a reading nobody asked for. `version.hasSample` on the
 *  row says whether this is worth calling at all, so `enabled` gates it.
 *
 *  `undefined` after a settled query means the version produced nothing — an ordinary state for a
 *  version, which the caller says in words rather than showing an empty preview. */
export function useVoiceVersionSample(
  ownerId: string,
  voiceId: string,
  version: bigint | undefined,
): { sample: VoiceVersionSample | undefined; isPending: boolean; isError: boolean } {
  const transport = useTransport()
  const query = useQuery({
    queryKey: voiceVersionSampleQueryKey(transport, ownerId, voiceId, version ?? 0n),
    queryFn: () =>
      createClient(VoiceService, transport).getVoiceProfileVersionSample({
        voiceId,
        version: version!,
      }),
    enabled: ownerId !== '' && voiceId !== '' && version !== undefined,
  })
  const content = query.data?.sample
  return {
    sample: content ? { content, createdAt: query.data?.createdAt ?? '' } : undefined,
    isPending: query.isPending,
    isError: query.isError,
  }
}
