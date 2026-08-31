import { useMutation, useQuery, useTransport } from '@connectrpc/connect-query'
import { createConnectQueryKey } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { AdminService, appFailureFromConnect } from '@/shared/api'
import type { PlanAccount, PlanName } from '../model/types'
import { planToProto, toPlanAccount } from './plan-mappers'

/** Every account and its tier. Master-only on the server, so a non-master caller gets a
 *  refusal here rather than an empty list — the screen that mounts this is itself gated. */
export function useAccounts(): {
  accounts: PlanAccount[]
  isPending: boolean
  isError: boolean
} {
  const { data, isPending, isError } = useQuery(AdminService.method.listUsers, {})
  return { accounts: (data?.users ?? []).map(toPlanAccount), isPending, isError }
}

/** Moves one account to another tier.
 *
 *  The list is invalidated rather than patched: the server refuses to demote the last
 *  master, so a local patch would show a change the database did not make. */
export function useSetUserPlan() {
  const queryClient = useQueryClient()
  const transport = useTransport()
  const mutation = useMutation(AdminService.method.setUserPlan, {
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: listUsersQueryKey(transport) })
    },
  })

  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    setPlan: (userId: string, plan: PlanName) =>
      mutation.mutate({ userId, plan: planToProto(plan) }),
  }
}

function listUsersQueryKey(transport: ReturnType<typeof useTransport>) {
  return createConnectQueryKey({
    schema: AdminService.method.listUsers,
    input: {},
    transport,
    cardinality: 'finite',
  })
}
