import { useMemo } from 'react'
import { createClient, type Transport } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import { GuidelineService } from '@/shared/api'
import type { GuidelineCandidate } from '../model/types'
import { guidelineCandidatesQueryKey, toGuidelineCandidate } from './guideline-queries'

/** The pending candidates, in the server's review order. A read and nothing else: mounting it
 *  records no candidate, calls no model and starts no job ([I5]).
 *
 *  `staleTime: 0` + `refetchOnMount: 'always'` against the app's 60s default, like the saved list:
 *  a candidate arrives from a revision the user ran in another tab, so a cached empty list is the
 *  wrong answer to "what is waiting for me". */
export function guidelineCandidateListQuery(transport: Transport, ownerId: string) {
  return {
    queryKey: guidelineCandidatesQueryKey(transport, ownerId),
    queryFn: () => createClient(GuidelineService, transport).listGuidelineCandidates({}),
    staleTime: 0,
    refetchOnMount: 'always' as const,
  }
}

export function useGuidelineCandidates(ownerId: string): {
  candidates: GuidelineCandidate[]
  /** The pending queue is at its server-side bound, so further revisions record nothing. The
   *  client owns no copy of the bound; this is the server's answer. */
  queueFull: boolean
  isPending: boolean
  isError: boolean
  isFetching: boolean
  refetch: () => void
} {
  const transport = useTransport()
  const query = useQuery({
    ...guidelineCandidateListQuery(transport, ownerId),
    enabled: ownerId !== '',
  })
  // The server returns them in review order — most-repeated first, then most recent — and the
  // client never reorders them.
  const candidates = useMemo(
    () => query.data?.candidates.map(toGuidelineCandidate) ?? [],
    [query.data],
  )
  return {
    candidates,
    queueFull: query.data?.queueFull ?? false,
    isPending: query.isPending,
    isError: query.isError,
    isFetching: query.isFetching,
    refetch: () => void query.refetch(),
  }
}
