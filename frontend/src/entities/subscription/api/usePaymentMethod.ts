import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { myPlanQueryKey } from '@/entities/plan/@x/subscription'
import { appFailureFromConnect, BillingService } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import { myBillingQueryKey } from './useMyBilling'

export function useRegisterPaymentMethod() {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(BillingService.method.registerPaymentMethod, {
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: myBillingQueryKey(transport) }),
        queryClient.invalidateQueries({ queryKey: myPlanQueryKey(transport) }),
      ])
    },
  })
  return {
    ...mutation,
    register: (authKey: string, customerKey: string) =>
      mutation.mutateAsync({ authKey, customerKey }),
    errorMessage: mutation.error ? formatAppFailure(appFailureFromConnect(mutation.error)) : '',
  }
}

export function useRemovePaymentMethod() {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(BillingService.method.removePaymentMethod, {
    onSuccess: () => queryClient.invalidateQueries({ queryKey: myBillingQueryKey(transport) }),
  })
  return {
    ...mutation,
    remove: () => mutation.mutateAsync({}),
    errorMessage: mutation.error ? formatAppFailure(appFailureFromConnect(mutation.error)) : '',
  }
}
