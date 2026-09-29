import type { Transport } from '@connectrpc/connect'
import type { QueryClient } from '@tanstack/react-query'
import { useCallback } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import {
  getComparisonPairsQueryKey,
  getSelectionsQueryKey,
  listModelsQueryKey,
} from './catalog-mappers'

/** A plan or balance change alters access projections without changing model registrations. */
export async function invalidateModelAccess(
  cache: QueryClient,
  transport: Transport,
): Promise<void> {
  await Promise.all([
    cache.invalidateQueries({ queryKey: listModelsQueryKey(transport) }),
    cache.invalidateQueries({ queryKey: getSelectionsQueryKey(transport) }),
    cache.invalidateQueries({ queryKey: getComparisonPairsQueryKey(transport) }),
  ])
}

export function useInvalidateModelAccess(): () => Promise<void> {
  const transport = useTransport()
  const cache = useQueryClient()
  return useCallback(() => invalidateModelAccess(cache, transport), [cache, transport])
}
