import { useMutation, useQuery, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { myPlanQueryKey } from '@/entities/plan/@x/subscription'
import { invalidateModelAccess } from '@/entities/model-catalog/@x/subscription'
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

export function useQuotePack(packId: string) {
  const query = useQuery(
    BillingService.method.quotePurchase,
    { packId },
    { enabled: packId.length > 0 },
  )
  return { ...query, quote: toPurchaseQuote(query.data, 0) }
}

function usePurchaseInvalidation() {
  const transport = useTransport()
  const queryClient = useQueryClient()
  return async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: myBillingQueryKey(transport) }),
      queryClient.invalidateQueries({ queryKey: myPlanQueryKey(transport) }),
      invalidateModelAccess(queryClient, transport),
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
    purchasePack: async (packId: string) => {
      const response = await mutation.mutateAsync({ packId })
      return response.purchase ? toPurchase(response.purchase) : undefined
    },
    errorMessage: errorMessage(mutation.error),
  }
}
