import { useQuery } from '@connectrpc/connect-query'
import { BillingService } from '@/shared/api'
import { toMyBilling } from './billing-mappers'

export function useMyBilling() {
  const query = useQuery(BillingService.method.getMyBilling, {})
  return { ...query, myBilling: toMyBilling(query.data) }
}
