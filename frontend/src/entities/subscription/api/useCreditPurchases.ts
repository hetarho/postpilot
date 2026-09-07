import { useMutation, useQuery, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { myPlanQueryKey } from '@/entities/plan/@x/subscription'
import { appFailureFromConnect, BillingService } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import { toPurchase, toPurchaseQuote } from './billing-mappers'
import { myBillingQueryKey } from './useMyBilling'

export function useQuotePurchase(usdCents: number) {
  const query = useQuery(
    BillingService.method.quotePurchase,
    { usdCents },
    { enabled: usdCents >= 100 },
  )
  return { ...query, quote: toPurchaseQuote(query.data, usdCents) }
}

function usePurchaseInvalidation() {
  const transport = useTransport()
  const queryClient = useQueryClient()
  return async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: myBillingQueryKey(transport) }),
      queryClient.invalidateQueries({ queryKey: myPlanQueryKey(transport) }),
    ])
  }
}

function errorMessage(error: unknown) {
  return error ? formatAppFailure(appFailureFromConnect(error)) : ''
}

export function usePurchaseCredits() {
  const mutation = useMutation(BillingService.method.purchaseCredits, {
    onSuccess: usePurchaseInvalidation(),
  })
  return {
    ...mutation,
    purchaseCredits: async (usdCents: number) => {
      const response = await mutation.mutateAsync({ usdCents })
      return response.purchase ? toPurchase(response.purchase) : undefined
    },
    errorMessage: errorMessage(mutation.error),
  }
}

export function useRefundPurchase() {
  const mutation = useMutation(BillingService.method.refundPurchase, {
    onSuccess: usePurchaseInvalidation(),
  })
  return {
    ...mutation,
    refundPurchase: async (purchaseId: string) => {
      const response = await mutation.mutateAsync({ purchaseId })
      return response.purchase ? toPurchase(response.purchase) : undefined
    },
    errorMessage: errorMessage(mutation.error),
  }
}
