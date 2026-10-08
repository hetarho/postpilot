import type { Transport } from '@connectrpc/connect'
import { createConnectQueryKey } from '@connectrpc/connect-query'
import type { QueryClient } from '@tanstack/react-query'
import { ModelCatalogService } from '@/shared/api'
import type { ModelPurpose } from '../config'
import { listRecommendationSetsQueryKey } from './catalog-mappers'
import { invalidateModelAccess } from './model-access-cache'

export function browseQueryKey(transport: Transport, purpose: ModelPurpose) {
  return createConnectQueryKey({
    schema: ModelCatalogService.method.listCatalog,
    input: { refresh: false, purpose },
    transport,
    cardinality: 'finite',
  })
}

/** Curation changes registrations and their projections, never the owner's saved choices. */
export async function invalidateCatalogViews(
  cache: QueryClient,
  transport: Transport,
  freshPurpose?: ModelPurpose,
): Promise<void> {
  const freshBrowse =
    freshPurpose === undefined
      ? undefined
      : cache.getQueryCache().find({
          queryKey: browseQueryKey(transport, freshPurpose),
          exact: true,
        })
  await Promise.all([
    cache.invalidateQueries({
      queryKey: createConnectQueryKey({
        schema: ModelCatalogService.method.listCatalog,
        transport,
        cardinality: 'finite',
      }),
      // A refresh already published this tab's actual response; all other purposes may drift.
      predicate: (query) => query !== freshBrowse,
    }),
    cache.invalidateQueries({
      queryKey: createConnectQueryKey({
        schema: ModelCatalogService.method.exportCatalogDocument,
        transport,
        cardinality: 'finite',
      }),
    }),
    cache.invalidateQueries({ queryKey: listRecommendationSetsQueryKey(transport) }),
    invalidateModelAccess(cache, transport),
  ])
}
