import { useMutation, useQuery } from '@connectrpc/connect-query'
import { BillingService } from '@/shared/api'
import { connectFailureMessage } from '@/shared/lib'
import { toPurchase, toPurchaseQuote } from './billing-mappers'
import { useBillingInvalidation } from './useBillingInvalidation'

export function useQuotePack(packId: string) {
  const query = useQuery(
    BillingService.method.quotePurchase,
    { packId },
    { enabled: packId.length > 0 },
  )
  return { ...query, quote: toPurchaseQuote(query.data) }
}

export function usePurchaseCredits() {
  const mutation = useMutation(BillingService.method.purchaseCredits, {
    onSuccess: useBillingInvalidation(),
  })
  return {
    ...mutation,
    purchasePack: async (packId: string) => {
      const response = await mutation.mutateAsync({ packId })
      return response.purchase ? toPurchase(response.purchase) : undefined
    },
    errorMessage: connectFailureMessage(mutation.error),
  }
}
