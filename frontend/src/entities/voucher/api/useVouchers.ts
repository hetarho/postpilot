import type { Transport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import {
  createConnectQueryKey,
  useMutation,
  useQuery,
  useTransport,
} from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import {
  ListVouchersResponseSchema,
  VoucherService,
  appFailureFromConnect,
  type ListVouchersResponse,
} from '@/shared/api'
import type { Voucher, VoucherIssue, VoucherPreset } from '../model/types'
import { toVoucher, toVoucherPreset, voucherIssueToProto } from './voucher-mappers'

function listVouchersQueryKey(transport: Transport) {
  return createConnectQueryKey({
    schema: VoucherService.method.listVouchers,
    input: {},
    transport,
    cardinality: 'finite',
  })
}

/** Every voucher, newest first, and the issue presets (GIFT-3, GIFT-14). Master-only on the
 *  server, so any other caller gets a refusal rather than an empty list. */
export function useVouchers(): {
  vouchers: Voucher[]
  presets: VoucherPreset[]
  isPending: boolean
  isError: boolean
} {
  const { data, isPending, isError } = useQuery(VoucherService.method.listVouchers, {})
  return {
    vouchers: (data?.vouchers ?? []).map(toVoucher),
    presets: (data?.presets ?? []).map(toVoucherPreset),
    isPending,
    isError,
  }
}

/** Issues one voucher and refreshes the list the new row belongs to. */
export function useIssueVoucher() {
  const queryClient = useQueryClient()
  const transport = useTransport()
  const mutation = useMutation(VoucherService.method.issueVoucher, {
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: listVouchersQueryKey(transport) })
    },
  })
  return {
    isPending: mutation.isPending,
    issued: mutation.data?.voucher ? toVoucher(mutation.data.voucher) : undefined,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    issue: (issue: VoucherIssue) => mutation.mutate(voucherIssueToProto(issue)),
    reset: mutation.reset,
  }
}

/** Revokes one voucher and puts the row the server returned in place of the old one. */
export function useRevokeVoucher() {
  const queryClient = useQueryClient()
  const transport = useTransport()
  const mutation = useMutation(VoucherService.method.revokeVoucher, {
    onSuccess: (response) => {
      const revoked = response.voucher
      if (!revoked) return
      queryClient.setQueryData(
        listVouchersQueryKey(transport),
        (current: ListVouchersResponse | undefined) =>
          current
            ? create(ListVouchersResponseSchema, {
                ...current,
                vouchers: current.vouchers.map((voucher) =>
                  voucher.id === revoked.id ? revoked : voucher,
                ),
              })
            : current,
      )
    },
  })
  return {
    isPending: mutation.isPending,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    revoke: (id: string) => mutation.mutateAsync({ id }),
  }
}
