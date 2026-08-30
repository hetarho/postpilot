import { useMemo } from 'react'
import { createClient, type Transport } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import { PurposeService } from '@/shared/api'
import type { Purpose } from '../model/types'
import { purposesQueryKey, toPurpose } from './purpose-queries'

/** The one query behind the directory, shared by the management screen and every selector.
 *  A read and nothing else: mounting it creates no purpose and starts no job ([I5]).
 *
 *  `staleTime: 0` + `refetchOnMount: 'always'` against the app's 60s default, because
 *  `post_count` is a projection over POSTS: assigning a purpose in the editor changes it
 *  without touching any purpose, so no purpose mutation can invalidate it. A cached count is
 *  what the delete confirmation would otherwise state — the one number that must not be a
 *  guess, since the user confirms a detach against it. */
export function purposeDirectoryQuery(transport: Transport, ownerId: string) {
  return {
    queryKey: purposesQueryKey(transport, ownerId),
    queryFn: () => createClient(PurposeService, transport).listPurposes({}),
    staleTime: 0,
    refetchOnMount: 'always' as const,
  }
}

export function usePurposes(ownerId: string): {
  purposes: Purpose[]
  isPending: boolean
  isError: boolean
  isFetching: boolean
  refetch: () => void
} {
  const transport = useTransport()
  const query = useQuery({ ...purposeDirectoryQuery(transport, ownerId), enabled: ownerId !== '' })
  const purposes = useMemo(() => query.data?.purposes.map(toPurpose) ?? [], [query.data])
  return {
    purposes,
    isPending: query.isPending,
    isError: query.isError,
    isFetching: query.isFetching,
    refetch: () => void query.refetch(),
  }
}
