import { create } from '@bufbuild/protobuf'
import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo } from 'react'
import {
  appFailureFromConnect,
  type GetSelectionsResponse,
  GetSelectionsResponseSchema,
  ProviderService,
} from '@/shared/api'
import { getSelectionsQueryKey } from './catalog-mappers'

/** One preparation per active account observer. The private query is owner- and
 * transport-scoped; only its current observer publishes into the shared selection cache. */
export function useInitializeDefaultSelections(ownerId: string) {
  const transport = useTransport()
  const cache = useQueryClient()
  const client = useMemo(() => createClient(ProviderService, transport), [transport])
  const selectionKey = useMemo(() => getSelectionsQueryKey(transport), [transport])
  const query = useQuery({
    queryKey: ['initialize-model-defaults', ownerId, ...selectionKey],
    queryFn: async ({ signal }) => {
      const baseline = cache.getQueryData<GetSelectionsResponse>(selectionKey)
      const response = await client.initializeDefaultSelections({}, { signal })
      return { response, baseline }
    },
    enabled: !!ownerId,
    retry: false,
    staleTime: Infinity,
    gcTime: 0,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  })
  const prepared = query.data
  useEffect(() => {
    if (!ownerId || !prepared || query.isFetching) return
    let current = true
    // A selection read that began before preparation must not later erase the defaults.
    void cache.cancelQueries({ queryKey: selectionKey, exact: true }).then(() => {
      if (!current) return
      cache.setQueryData<GetSelectionsResponse>(selectionKey, (latest) => {
        const selections = [...prepared.response.selections]
        // A manual choice saved while preparation was running is newer than its answer.
        // Keep that choice while using the server's actual result for untouched stages.
        for (const selection of latest?.selections ?? []) {
          const before = prepared.baseline?.selections.find(
            (candidate) => candidate.stage === selection.stage,
          )
          if (
            before?.ref?.providerId === selection.ref?.providerId &&
            before?.ref?.modelId === selection.ref?.modelId
          )
            continue
          const index = selections.findIndex((candidate) => candidate.stage === selection.stage)
          if (index >= 0) selections[index] = selection
          else selections.push(selection)
        }
        return create(GetSelectionsResponseSchema, { selections })
      })
    })
    return () => {
      current = false
    }
  }, [cache, ownerId, prepared, query.isFetching, selectionKey])
  const isPending = !!ownerId && (query.isPending || query.isFetching)
  const isError = !!ownerId && query.isError
  return {
    phase: isPending ? ('checking' as const) : isError ? ('failed' as const) : ('ready' as const),
    isPending,
    isError,
    failure: query.error ? appFailureFromConnect(query.error) : undefined,
    retry: () => {
      if (ownerId) void query.refetch()
    },
  }
}
