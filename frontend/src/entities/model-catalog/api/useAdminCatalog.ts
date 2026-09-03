import { useMutation, useQuery, useTransport } from '@connectrpc/connect-query'
import { createConnectQueryKey } from '@connectrpc/connect-query'
import { useQueryClient, type QueryClient } from '@tanstack/react-query'
import type { Transport } from '@connectrpc/connect'
import { ModelCatalogService, ProviderService, appFailureFromConnect } from '@/shared/api'
import type { CatalogBrowse, ReasoningEffortName } from '../model/types'
import { toCatalogBrowse } from './catalog-mappers'

const EMPTY: CatalogBrowse = { entries: [], fetchedAt: '', fromCache: false, fetchError: '' }

/** The operator's catalog. Master-only on the server, so a non-master caller is refused here
 *  rather than shown an empty list — the screen that mounts this is itself gated.
 *
 *  The query always asks with `refresh: false`; bypassing the server's cache is the separate
 *  `refresh` action below, because a cache-busting read is something the operator DOES, not a
 *  property of the screen being open. */
export function useAdminCatalog(): {
  catalog: CatalogBrowse
  isPending: boolean
  isError: boolean
} {
  const { data, isPending, isError } = useQuery(ModelCatalogService.method.listCatalog, {
    refresh: false,
  })
  return { catalog: data ? toCatalogBrowse(data) : EMPTY, isPending, isError }
}

/** Re-reads the provider's own catalog, past every cache. The answer replaces the browse
 *  query's data instead of invalidating it: the response IS the fresh list, so refetching
 *  after it would spend a second round trip to be told the same thing. */
export function useRefreshCatalog() {
  const queryClient = useQueryClient()
  const transport = useTransport()
  const mutation = useMutation(ModelCatalogService.method.listCatalog, {
    onSuccess: (data) => {
      queryClient.setQueryData(browseQueryKey(transport), data)
    },
  })
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    refresh: () => mutation.mutate({ refresh: true }),
  }
}

/** Makes a model selectable at an explicitly chosen floor. */
export function useEnableModel() {
  const queryClient = useQueryClient()
  const transport = useTransport()
  const mutation = useMutation(ModelCatalogService.method.enableModel, {
    onSettled: () => invalidateCatalogs(queryClient, transport),
  })
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    enable: (modelId: string) => mutation.mutate({ modelId }),
  }
}

/** Changes one curated model. Only the named properties are sent, so two edits to different
 *  properties of one model do not overwrite each other. */
export function useUpdateModel() {
  const queryClient = useQueryClient()
  const transport = useTransport()
  const mutation = useMutation(ModelCatalogService.method.updateModel, {
    onSettled: () => invalidateCatalogs(queryClient, transport),
  })
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    update: (
      modelId: string,
      patch: { enabled?: boolean; reasoningEffort?: ReasoningEffortName },
    ) =>
      mutation.mutate({
        modelId,
        enabled: patch.enabled,
        reasoningEffort: patch.reasoningEffort,
      }),
  }
}

/** A curation write changes what EVERY account may select, so the user-facing catalog is
 *  invalidated beside the operator's own list — otherwise the model picker keeps serving a
 *  five-minute-stale list to the very operator who just changed it. */
function invalidateCatalogs(queryClient: QueryClient, transport: Transport) {
  void queryClient.invalidateQueries({ queryKey: browseQueryKey(transport) })
  void queryClient.invalidateQueries({
    queryKey: createConnectQueryKey({
      schema: ProviderService.method.listModels,
      input: {},
      transport,
      cardinality: 'finite',
    }),
  })
}

function browseQueryKey(transport: Transport) {
  return createConnectQueryKey({
    schema: ModelCatalogService.method.listCatalog,
    input: { refresh: false },
    transport,
    cardinality: 'finite',
  })
}
