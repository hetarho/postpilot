import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { BillingService } from '@/shared/api'
import { connectFailureMessage } from '@/shared/lib'
import { useBillingInvalidation } from './useBillingInvalidation'
import { myBillingQueryKey } from './useMyBilling'

export function useRegisterPaymentMethod() {
  const mutation = useMutation(BillingService.method.registerPaymentMethod, {
    onSuccess: useBillingInvalidation(),
  })
  return {
    ...mutation,
    register: (authKey: string, customerKey: string) =>
      mutation.mutateAsync({ authKey, customerKey }),
    errorMessage: connectFailureMessage(mutation.error),
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
    errorMessage: connectFailureMessage(mutation.error),
  }
}
