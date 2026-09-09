import { useMutation, useQuery, useTransport } from '@connectrpc/connect-query'
import { createConnectQueryKey } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { ModelCatalogService, ProviderService, appFailureFromConnect } from '@/shared/api'
import { toCatalogDocumentPlan } from './catalog-mappers'

/** The current registrations of all five purposes, rendered as the paste protocol. It is what
 *  the operator edits instead of writing a document from memory, so it is read when the panel
 *  opens rather than on every catalog screen — `enabled` carries that. */
export function useCatalogDocument(enabled: boolean): {
  document: string
  isPending: boolean
  isError: boolean
} {
  const { data, isPending, isError } = useQuery(
    ModelCatalogService.method.exportCatalogDocument,
    {},
    { enabled },
  )
  return { document: data?.document ?? '', isPending: enabled && isPending, isError }
}

/** Reads a pasted document and reports what applying it would do. It writes nothing, so it is
 *  safe to run on every 미리보기 press. */
export function usePreviewCatalogDocument() {
  const mutation = useMutation(ModelCatalogService.method.previewCatalogDocument)
  return {
    ...mutation,
    plan: mutation.data ? toCatalogDocumentPlan(mutation.data) : undefined,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    preview: (document: string) => mutation.mutate({ document }),
    reset: mutation.reset,
  }
}

/** Applies the document. The server validates the text again from scratch, so what goes up is
 *  the text itself and never the preview's answer — the catalog moves between the two calls. */
export function useApplyCatalogDocument() {
  const queryClient = useQueryClient()
  const transport = useTransport()
  const mutation = useMutation(ModelCatalogService.method.applyCatalogDocument, {
    onSettled: () => {
      // A sync changes what every account may select, and it can touch every purpose at once,
      // so both the operator's per-purpose listings and the user-facing catalog are dropped.
      void queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({
          schema: ModelCatalogService.method.listCatalog,
          transport,
          cardinality: 'finite',
        }),
      })
      void queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({
          schema: ModelCatalogService.method.exportCatalogDocument,
          transport,
          cardinality: 'finite',
        }),
      })
      void queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({
          schema: ProviderService.method.listModels,
          input: {},
          transport,
          cardinality: 'finite',
        }),
      })
    },
  })
  return {
    ...mutation,
    result: mutation.data ? toCatalogDocumentPlan(mutation.data) : undefined,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    apply: (document: string) => mutation.mutate({ document }),
    reset: mutation.reset,
  }
}
