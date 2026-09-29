import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { VoiceService } from '@/shared/api'
import type { PostFingerprint } from '../model/fingerprint'
import { toComparisons } from './fingerprint-comparison'
import { postFingerprintQueryKey } from './voice-queries'

/** This post's fingerprint beside its voice's at `revision` (POST-102). A new revision — a
 *  generation, an AI 수정 or a hand edit — reads anew, and the previous reading stays on screen
 *  while it loads. */
export function usePostFingerprint(
  ownerId: string,
  slug: string,
  revision: bigint,
  enabled = true,
): {
  fingerprint: PostFingerprint | undefined
  isError: boolean
  isFetching: boolean
  refetch: () => void
} {
  const transport = useTransport()
  const query = useQuery({
    queryKey: postFingerprintQueryKey(transport, ownerId, slug, revision),
    queryFn: () =>
      createClient(VoiceService, transport)
        .getPostFingerprint({ postSlug: slug })
        .then((response): PostFingerprint => ({
          applicable: response.applicable,
          revision: response.revision,
          items: toComparisons(response.items),
        })),
    enabled: enabled && ownerId !== '' && slug !== '',
    placeholderData: keepPreviousData,
  })
  return {
    fingerprint: query.data,
    isError: query.isError,
    isFetching: query.isFetching,
    refetch: () => void query.refetch(),
  }
}
