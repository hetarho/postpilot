import type { Transport } from '@connectrpc/connect'
import { createConnectQueryKey, useQuery } from '@connectrpc/connect-query'
import { BillingService } from '@/shared/api'
import { toMyBilling } from './billing-mappers'

export function useMyBilling() {
  const query = useQuery(BillingService.method.getMyBilling, {})
  return { ...query, myBilling: toMyBilling(query.data) }
}

export function myBillingQueryKey(transport: Transport) {
  return createConnectQueryKey({
    schema: BillingService.method.getMyBilling,
    input: {},
    transport,
    cardinality: 'finite',
  })
}
