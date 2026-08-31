import { useMemo } from 'react'
import { createClient, type Transport } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import { GuidelineService } from '@/shared/api'
import type { Guideline } from '../model/types'
import { guidelinesQueryKey, toGuideline } from './guideline-queries'

/** The one query behind the list. A read and nothing else: mounting it creates no guideline,
 *  calls no model and starts no job ([I5]).
 *
 *  `staleTime: 0` + `refetchOnMount: 'always'` against the app's 60s default, like the purpose
 *  directory, because each scoped guideline's purpose names are a PROJECTION: renaming a purpose
 *  changes what this list shows without touching any guideline row. */
export function guidelineListQuery(transport: Transport, ownerId: string) {
  return {
    queryKey: guidelinesQueryKey(transport, ownerId),
    queryFn: () => createClient(GuidelineService, transport).listGuidelines({}),
    staleTime: 0,
    refetchOnMount: 'always' as const,
  }
}

export function useGuidelines(ownerId: string): {
  guidelines: Guideline[]
  isPending: boolean
  isError: boolean
  isFetching: boolean
  refetch: () => void
} {
  const transport = useTransport()
  const query = useQuery({ ...guidelineListQuery(transport, ownerId), enabled: ownerId !== '' })
  // The server returns them in injection order; the client never reorders them, so the screen
  // shows exactly the order the writer will be given.
  const guidelines = useMemo(() => query.data?.guidelines.map(toGuideline) ?? [], [query.data])
  return {
    guidelines,
    isPending: query.isPending,
    isError: query.isError,
    isFetching: query.isFetching,
    refetch: () => void query.refetch(),
  }
}
