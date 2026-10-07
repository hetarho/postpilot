import { useCallback, useEffect, useMemo, useSyncExternalStore } from 'react'
import { useQuery, useTransport } from '@connectrpc/connect-query'
import { hashKey, notifyManager, useQueryClient } from '@tanstack/react-query'
import { myPlanQueryKey } from '@/entities/plan/@x/model-catalog'
import { ProviderService } from '@/shared/api'
import { MODEL_CATALOG_STALE_MS } from '../config'
import type { CatalogModel } from '../model/types'
import { toCatalogModel } from './catalog-mappers'
import { listModelsQueryKey } from './catalog-mappers'

const NONE: CatalogModel[] = []

/** The registry snapshot, in the yaml's order. */
export function useModels(): {
  models: readonly CatalogModel[]
  isPending: boolean
  isError: boolean
  refetch: () => void
} {
  const transport = useTransport()
  const cache = useQueryClient()
  const planKey = useMemo(() => myPlanQueryKey(transport), [transport])
  const modelKey = useMemo(() => listModelsQueryKey(transport), [transport])
  const subscribe = useCallback(
    (notify: () => void) =>
      cache.getQueryCache().subscribe(
        notifyManager.batchCalls((event) => {
          // Child observers are attached during render. Only changes to the plan
          // snapshot affect this projection, and their notifications follow query batching.
          if (
            event.query.queryHash === hashKey(planKey) &&
            (event.type === 'updated' || event.type === 'removed')
          )
            notify()
        }),
      ),
    [cache, planKey],
  )
  const planSnapshot = useCallback(
    () => cache.getQueryState(planKey)?.dataUpdatedAt ?? 0,
    [cache, planKey],
  )
  const planUpdatedAt = useSyncExternalStore(subscribe, planSnapshot, planSnapshot)
  const { data, dataUpdatedAt, isPending, isError, refetch } = useQuery(
    ProviderService.method.listModels,
    {},
    { staleTime: MODEL_CATALOG_STALE_MS },
  )
  // Credits and plan ceilings are projected into ListModels. A new plan response stales that
  // projection even if the operator's catalog itself is still inside its five-minute cache.
  useEffect(() => {
    if (dataUpdatedAt > 0 && planUpdatedAt > dataUpdatedAt)
      void cache.invalidateQueries({ queryKey: modelKey })
  }, [cache, dataUpdatedAt, modelKey, planUpdatedAt])
  const models = useMemo(() => data?.models.map(toCatalogModel) ?? NONE, [data])
  return {
    models,
    isPending,
    isError,
    refetch: () => {
      void refetch()
    },
  }
}
