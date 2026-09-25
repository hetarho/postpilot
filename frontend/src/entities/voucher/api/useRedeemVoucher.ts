import { useMutation } from '@connectrpc/connect-query'
import { VoucherService, appFailureFromConnect, type AppFailure } from '@/shared/api'
import type { Redemption } from '../model/types'
import { toRedemption } from './voucher-mappers'

/** Redeems a gift link for the signed-in account (GIFT-9). The caller decides what to
 *  refresh afterwards: the balance lives in another entity. */
export function useRedeemVoucher(
  options: {
    onRedeemed?: (redemption: Redemption) => void
    onRefused?: (failure: AppFailure) => void
  } = {},
) {
  const mutation = useMutation(VoucherService.method.redeemVoucher, {
    onSuccess: (response) => options.onRedeemed?.(toRedemption(response)),
    onError: (error) => options.onRefused?.(appFailureFromConnect(error)),
  })
  return {
    isPending: mutation.isPending,
    redemption: mutation.data ? toRedemption(mutation.data) : undefined,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    redeem: (token: string) => mutation.mutate({ token }),
  }
}
