import { useQuery } from '@connectrpc/connect-query'
import { planToProto, type PlanName } from '@/entities/plan/@x/subscription'
import { BillingService, ProtoPlan, ProtoTerm } from '@/shared/api'
import type { BillingTerm } from '../model/types'
import { billablePlan, termToProto, toQuote } from './billing-mappers'

export function useQuote(
  plan: PlanName | undefined,
  term: BillingTerm | undefined,
  queryEnabled = true,
) {
  const enabled = queryEnabled && billablePlan(plan) && term !== undefined
  const query = useQuery(
    BillingService.method.quotePrice,
    {
      plan: enabled ? planToProto(plan) : ProtoPlan.UNSPECIFIED,
      term: enabled ? termToProto(term) : ProtoTerm.UNSPECIFIED,
    },
    { enabled },
  )
  return { ...query, quote: toQuote(query.data) }
}
