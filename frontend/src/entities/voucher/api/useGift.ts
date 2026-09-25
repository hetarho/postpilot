import { useMemo } from 'react'
import { createConnectQueryKey, useQuery, useTransport } from '@connectrpc/connect-query'
import { VoucherService, appFailureFromConnect, type AppFailure } from '@/shared/api'
import type { GiftView } from '../model/types'
import { toGiftView } from './voucher-mappers'

/** The public read behind a gift link (GIFT-8). It needs no session, so the gift page can
 *  show what the link holds before the visitor has an account. */
export function useGift(token: string): {
  gift: GiftView | undefined
  isPending: boolean
  failure: AppFailure | undefined
} {
  const { data, isPending, error } = useQuery(
    VoucherService.method.getVoucher,
    { token },
    { enabled: token !== '' },
  )
  return {
    gift: data ? toGiftView(data) : undefined,
    isPending: token !== '' && isPending,
    failure: error ? appFailureFromConnect(error) : undefined,
  }
}

/** The gift read's cache entry, for a redemption that must refetch it. */
export function useGiftQueryKey(token: string) {
  const transport = useTransport()
  return useMemo(
    () =>
      createConnectQueryKey({
        schema: VoucherService.method.getVoucher,
        input: { token },
        transport,
        cardinality: 'finite',
      }),
    [token, transport],
  )
}
