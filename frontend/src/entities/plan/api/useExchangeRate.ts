import { useQuery } from '@connectrpc/connect-query'
import { AdminService } from '@/shared/api'
import type { ExchangeRate } from '../model/types'
import { toExchangeRate } from './plan-mappers'

/** The rate a paid job admitted now would be priced at (QUOTA-59), for /admin's 비용·환율 tab.
 *  Master-only on the server; customer reads carry no rate, master included (QUOTA-65). */
export function useExchangeRate(): {
  exchangeRate: ExchangeRate | undefined
  isPending: boolean
  isError: boolean
  isFetching: boolean
  refetch: () => void
} {
  const { data, isPending, isError, isFetching, refetch } = useQuery(
    AdminService.method.getExchangeRate,
    {},
  )
  return {
    exchangeRate: toExchangeRate(data),
    isPending,
    isError,
    isFetching,
    refetch: () => void refetch(),
  }
}
