import { useMemo } from 'react'
import { createClient, type Transport } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import { MemoryService } from '@/shared/api'
import type { Memory } from '../model/types'
import { memoriesQueryKey, toMemory } from './memory-queries'

/** The one query behind the list. A read and nothing else: mounting it creates no memory, calls
 *  no model and starts no job ([I5], MEM-23).
 *
 *  `staleTime: 0` + `refetchOnMount: 'always'` against the app's 60s default, like the guideline
 *  directory: the order is the server's — most recently USED first — and a generation that
 *  selected a memory moves it without this screen touching anything. */
export function memoryListQuery(transport: Transport, ownerId: string) {
  return {
    queryKey: memoriesQueryKey(transport, ownerId),
    queryFn: () => createClient(MemoryService, transport).listMemories({}),
    staleTime: 0,
    refetchOnMount: 'always' as const,
  }
}

export function useMemories(ownerId: string): {
  memories: Memory[]
  isPending: boolean
  isError: boolean
  isFetching: boolean
  refetch: () => void
} {
  const transport = useTransport()
  const query = useQuery({ ...memoryListQuery(transport, ownerId), enabled: ownerId !== '' })
  // The server returns them in injection order; the client never reorders them, so the screen
  // shows exactly the order a post that opted in would be given.
  const memories = useMemo(() => query.data?.memories.map(toMemory) ?? [], [query.data])
  return {
    memories,
    isPending: query.isPending,
    isError: query.isError,
    isFetching: query.isFetching,
    refetch: () => void query.refetch(),
  }
}
