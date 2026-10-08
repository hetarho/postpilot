import { useMutation, useQuery, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { ModelCatalogService, appFailureFromConnect } from '@/shared/api'
import type { ModelPurpose } from '../config'
import type { CatalogBrowse, ReasoningEffortName } from '../model/types'
import type { LevelName } from '../model/level'
import { toCatalogBrowse } from './catalog-mappers'
import { browseQueryKey, invalidateCatalogViews } from './catalog-cache'

const EMPTY: CatalogBrowse = {
  entries: [],
  fetchedAt: '',
  fromCache: false,
  fetchError: '',
  estimatorCombos: [],
}

/** The operator's catalog. Master-only on the server, so a non-master caller is refused here
 *  rather than shown an empty list — the screen that mounts this is itself gated.
 *
 *  The query always asks with `refresh: false`; bypassing the server's cache is the separate
 *  `refresh` action below, because a cache-busting read is something the operator DOES, not a
 *  property of the screen being open.
 *
 *  `purpose` is the tab being read: the server reports each entry's effort for THAT purpose
 *  and attaches that stage's spend signal, so the control and the evidence beside it belong
 *  to the tab the operator is looking at (MODEL-28). It is part of the query key, so
 *  switching tabs is a different read rather than a re-render of the previous tab's answer. */
export function useAdminCatalog(purpose: ModelPurpose): {
  catalog: CatalogBrowse
  isPending: boolean
  isError: boolean
} {
  const { data, isPending, isError } = useQuery(ModelCatalogService.method.listCatalog, {
    refresh: false,
    purpose,
  })
  return { catalog: data ? toCatalogBrowse(data) : EMPTY, isPending, isError }
}

/** Re-reads the provider's own catalog, past every cache. The answer replaces the browse
 *  query's data instead of invalidating it: the response IS the fresh list, so refetching
 *  after it would spend a second round trip to be told the same thing. */
export function useRefreshCatalog(purpose: ModelPurpose) {
  const queryClient = useQueryClient()
  const transport = useTransport()
  const mutation = useMutation(ModelCatalogService.method.listCatalog, {
    onSuccess: async (data) => {
      queryClient.setQueryData(browseQueryKey(transport, purpose), data)
      await invalidateCatalogViews(queryClient, transport, purpose)
    },
  })
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    refresh: () => mutation.mutate({ refresh: true, purpose }),
  }
}

/** Registers or deregisters a model for ONE purpose — the active tab's checkbox. The server
 *  enforces the purpose's capability gate, so the hidden-ineligible-rows rendering is a
 *  convenience, not the enforcement. */
export function useSetModelPurpose() {
  const queryClient = useQueryClient()
  const transport = useTransport()
  const mutation = useMutation(ModelCatalogService.method.setModelPurpose, {
    onSettled: () => invalidateCatalogViews(queryClient, transport),
  })
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    setPurpose: (modelId: string, purpose: ModelPurpose, registered: boolean) =>
      mutation.mutate({ modelId, purpose, registered }),
  }
}

/** Changes one curated model. Only the named properties are sent, so two edits to different
 *  properties of one model do not overwrite each other. */
export function useUpdateModel() {
  const queryClient = useQueryClient()
  const transport = useTransport()
  const mutation = useMutation(ModelCatalogService.method.updateModel, {
    onSettled: () => invalidateCatalogViews(queryClient, transport),
  })
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    // The purpose is required: the effort belongs to a REGISTRATION, and the server refuses
    // one the model does not serve.
    update: (
      modelId: string,
      purpose: ModelPurpose,
      // Both halves are optional and independent: a request that names only one leaves the
      // other exactly as it was, which is why the proto fields are `optional` (T092).
      patch: { reasoningEffort?: ReasoningEffortName; level?: LevelName | '' },
    ) =>
      mutation.mutate({
        modelId,
        purpose,
        reasoningEffort: patch.reasoningEffort,
        level: patch.level,
      }),
  }
}
