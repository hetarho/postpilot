import { useMutation, useTransport } from '@connectrpc/connect-query'
import { createConnectQueryKey } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { AdminService, ModelCatalogService, PlanService, appFailureFromConnect } from '@/shared/api'

/** Points one estimator combo at the two models that price it.
 *
 *  Both refs go in one call: the server takes a complete assignment, so a half-chosen combo
 *  is client state that has not been sent rather than a row with one model.
 *
 *  A success invalidates the operator's catalog (which carries the assignments) and every
 *  account's plan read — the comparison screen's post counts are derived from these rates, so
 *  leaving GetMyPlan cached would keep quoting the tier that was just replaced. */
export function useAssignEstimatorCombo() {
  const queryClient = useQueryClient()
  const transport = useTransport()
  const mutation = useMutation(AdminService.method.setEstimatorCombo, {
    onSettled: () => {
      void queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({
          schema: ModelCatalogService.method.listCatalog,
          transport,
          cardinality: 'finite',
        }),
      })
      void queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({
          schema: PlanService.method.getMyPlan,
          input: {},
          transport,
          cardinality: 'finite',
        }),
      })
    },
  })
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    assign: (combo: string, observeModelId: string, writeModelId: string) =>
      mutation.mutate({ combo, observeModelId, writeModelId }),
  }
}
