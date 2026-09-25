import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import {
  GetVoucherResponseSchema,
  IssueVoucherResponseSchema,
  ListVouchersResponseSchema,
  ProtoPlan,
  ProtoVoucherState,
  RedeemVoucherResponseSchema,
  RevokeVoucherResponseSchema,
  VoucherPresetSchema,
  VoucherSchema,
  VoucherService,
  type ProtoVoucher,
} from '@/shared/api'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

/** A voucher the fake starts with. Anything left out takes a redeemable pro-sized default. */
export interface FakeVoucherSeed {
  id?: string
  token: string
  credits?: number
  validityDays?: number
  message?: string
  state?: ProtoVoucherState
  issuedAt?: string
  linkExpiresAt?: string
  sale?: { amountKrw: number; payerName: string }
  redeemedBy?: string
  redeemedAt?: string
  remainingCredits?: number
  creditsExpireAt?: string
}

export interface FakeVoucherIssue {
  credits: number
  validityDays: number
  message: string
  sale?: { amountKrw: number; payerName: string }
}

export interface FakeVoucherOptions {
  calls?: string[]
  vouchers?: FakeVoucherSeed[]
  /** The public read fails as the network would, rather than with a refusal. */
  getUnavailable?: boolean
  redeemFailure?: 'VOUCHER_REDEEMED' | 'VOUCHER_EXPIRED' | 'VOUCHER_REVOKED'
  redeemRequests?: string[]
  issueRequests?: FakeVoucherIssue[]
  issueFailure?: 'VOUCHER_INVALID'
  revokeRequests?: string[]
  listUnavailable?: boolean
}

const PRESETS = [
  { plan: ProtoPlan.BASIC, credits: 330, validityDays: 30 },
  { plan: ProtoPlan.PRO, credits: 1150, validityDays: 30 },
  { plan: ProtoPlan.MAX, credits: 2400, validityDays: 30 },
]

function toVoucher(seed: FakeVoucherSeed, index: number): ProtoVoucher {
  return create(VoucherSchema, {
    id: seed.id ?? `voucher-${index + 1}`,
    token: seed.token,
    credits: seed.credits ?? 1150,
    validityDays: seed.validityDays ?? 30,
    message: seed.message ?? '',
    state: seed.state ?? ProtoVoucherState.REDEEMABLE,
    issuedAt: seed.issuedAt ?? '2026-09-25T03:00:00Z',
    linkExpiresAt: seed.linkExpiresAt ?? '2026-12-24T03:00:00Z',
    sale: seed.sale
      ? { amountKrw: BigInt(seed.sale.amountKrw), payerName: seed.sale.payerName }
      : undefined,
    redeemedBy: seed.redeemedBy ?? '',
    redeemedAt: seed.redeemedAt ?? '',
    remainingCredits: seed.remainingCredits ?? 0,
    creditsExpireAt: seed.creditsExpireAt ?? '',
  })
}

/** The voucher surface: the public gift read, redemption, and the operator's issue/list/revoke.
 *  It keeps its own list, so an issue shows up in the next list and a revoke changes its row. */
export function registerVoucherService(router: ConnectRouter, options: FakeVoucherOptions = {}) {
  const { calls } = options
  const vouchers = (options.vouchers ?? []).map(toVoucher)
  const find = (token: string) => vouchers.find((voucher) => voucher.token === token)

  router.rpc(VoucherService.method.getVoucher, (request) => {
    calls?.push('GetVoucher')
    if (options.getUnavailable) throw new ConnectError('offline', Code.Unavailable)
    const voucher = find(request.token)
    if (!voucher) throw connectAppError('VOUCHER_NOT_FOUND', Code.NotFound)
    return create(GetVoucherResponseSchema, {
      credits: voucher.credits,
      validityDays: voucher.validityDays,
      message: voucher.message,
      state: voucher.state,
      linkExpiresAt: voucher.linkExpiresAt,
    })
  })

  router.rpc(VoucherService.method.redeemVoucher, (request) => {
    calls?.push('RedeemVoucher')
    options.redeemRequests?.push(request.token)
    const voucher = find(request.token)
    if (!voucher) throw connectAppError('VOUCHER_NOT_FOUND', Code.NotFound)
    if (options.redeemFailure) {
      voucher.state =
        options.redeemFailure === 'VOUCHER_REDEEMED'
          ? ProtoVoucherState.REDEEMED
          : options.redeemFailure === 'VOUCHER_EXPIRED'
            ? ProtoVoucherState.EXPIRED
            : ProtoVoucherState.REVOKED
      throw connectAppError(options.redeemFailure, Code.FailedPrecondition)
    }
    voucher.state = ProtoVoucherState.REDEEMED
    voucher.creditsExpireAt = '2026-10-25T03:00:00Z'
    return create(RedeemVoucherResponseSchema, {
      credits: voucher.credits,
      creditsExpireAt: voucher.creditsExpireAt,
    })
  })

  router.rpc(VoucherService.method.issueVoucher, (request) => {
    calls?.push('IssueVoucher')
    options.issueRequests?.push({
      credits: request.credits,
      validityDays: request.validityDays,
      message: request.message,
      sale: request.sale
        ? { amountKrw: Number(request.sale.amountKrw), payerName: request.sale.payerName }
        : undefined,
    })
    if (options.issueFailure) throw connectAppError(options.issueFailure, Code.InvalidArgument)
    const voucher = toVoucher(
      {
        id: `voucher-${vouchers.length + 1}`,
        token: `issued-token-${vouchers.length + 1}`,
        credits: request.credits,
        validityDays: request.validityDays,
        message: request.message,
        sale: request.sale
          ? { amountKrw: Number(request.sale.amountKrw), payerName: request.sale.payerName }
          : undefined,
      },
      vouchers.length,
    )
    vouchers.unshift(voucher)
    return create(IssueVoucherResponseSchema, { voucher })
  })

  router.rpc(VoucherService.method.listVouchers, () => {
    calls?.push('ListVouchers')
    if (options.listUnavailable) throw new ConnectError('offline', Code.Unavailable)
    return create(ListVouchersResponseSchema, {
      vouchers: vouchers.map((voucher) =>
        create(VoucherSchema, {
          ...voucher,
          token: voucher.state === ProtoVoucherState.REDEEMABLE ? voucher.token : '',
        }),
      ),
      presets: PRESETS.map((preset) => create(VoucherPresetSchema, preset)),
    })
  })

  router.rpc(VoucherService.method.revokeVoucher, (request) => {
    calls?.push('RevokeVoucher')
    options.revokeRequests?.push(request.id)
    const voucher = vouchers.find((candidate) => candidate.id === request.id)
    if (!voucher) throw connectAppError('VOUCHER_NOT_FOUND', Code.NotFound)
    voucher.state = ProtoVoucherState.REVOKED
    voucher.remainingCredits = 0
    return create(RevokeVoucherResponseSchema, {
      voucher: create(VoucherSchema, { ...voucher, token: '' }),
    })
  })
}
