import { useEffect, useState } from 'react'
import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { VoiceService } from '@/shared/api'
import { AUTOSAVE_DEBOUNCE_MS } from '@/shared/config'
import type { PostFingerprint } from '../model/fingerprint'
import { toComparisons } from './fingerprint-comparison'
import { postFingerprintQueryKey } from './voice-queries'

/** The revision once it has stayed put for the autosave beat. Every ② block autosave moves the
 *  content revision, so reading at each one cost a request and a cache entry per save; a burst of
 *  saves now reads once, a beat after the last. The first revision is read at once. */
function useSettledRevision(revision: bigint): bigint {
  const [settled, setSettled] = useState(revision)
  useEffect(() => {
    if (revision === settled) return
    const timer = setTimeout(() => setSettled(revision), AUTOSAVE_DEBOUNCE_MS)
    return () => clearTimeout(timer)
  }, [revision, settled])
  return settled
}

/** This post's fingerprint beside its voice's at `revision` (POST-102). A new revision — a
 *  generation, an AI 수정 or a hand edit — reads anew once it settles, and the previous reading
 *  stays on screen until then and while it loads. */
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
  const settled = useSettledRevision(revision)
  const query = useQuery({
    queryKey: postFingerprintQueryKey(transport, ownerId, slug, settled),
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
