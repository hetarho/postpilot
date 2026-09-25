import {
  ProtoPlan,
  ProtoVoucherState,
  type GetVoucherResponse,
  type ProtoVoucher,
  type ProtoVoucherPreset,
  type RedeemVoucherResponse,
} from '@/shared/api'
import type {
  GiftView,
  Redemption,
  Voucher,
  VoucherIssue,
  VoucherPreset,
  VoucherState,
} from '../model/types'

export function toVoucherState(state: ProtoVoucherState): VoucherState {
  switch (state) {
    case ProtoVoucherState.REDEEMABLE:
      return 'redeemable'
    case ProtoVoucherState.REDEEMED:
      return 'redeemed'
    case ProtoVoucherState.EXPIRED:
      return 'expired'
    case ProtoVoucherState.REVOKED:
      return 'revoked'
    default:
      return 'unknown'
  }
}

export function toGiftView(response: GetVoucherResponse): GiftView {
  return {
    credits: response.credits,
    validityDays: response.validityDays,
    message: response.message,
    state: toVoucherState(response.state),
    linkExpiresAt: response.linkExpiresAt,
  }
}

export function toRedemption(response: RedeemVoucherResponse): Redemption {
  return { credits: response.credits, creditsExpireAt: response.creditsExpireAt }
}

export function toVoucher(voucher: ProtoVoucher): Voucher {
  return {
    id: voucher.id,
    credits: voucher.credits,
    validityDays: voucher.validityDays,
    sale: voucher.sale
      ? { amountKrw: Number(voucher.sale.amountKrw), payerName: voucher.sale.payerName }
      : undefined,
    message: voucher.message,
    state: toVoucherState(voucher.state),
    issuedAt: voucher.issuedAt,
    linkExpiresAt: voucher.linkExpiresAt,
    redeemedBy: voucher.redeemedBy,
    redeemedAt: voucher.redeemedAt,
    remainingCredits: voucher.remainingCredits,
    creditsExpireAt: voucher.creditsExpireAt,
    token: voucher.token,
  }
}

export function toVoucherPreset(preset: ProtoVoucherPreset): VoucherPreset {
  const plan =
    preset.plan === ProtoPlan.BASIC
      ? 'basic'
      : preset.plan === ProtoPlan.PRO
        ? 'pro'
        : preset.plan === ProtoPlan.MAX
          ? 'max'
          : 'other'
  return { plan, credits: preset.credits, validityDays: preset.validityDays }
}

export function voucherIssueToProto(issue: VoucherIssue) {
  return {
    credits: issue.credits,
    validityDays: issue.validityDays,
    message: issue.message,
    sale: issue.sale
      ? { amountKrw: BigInt(issue.sale.amountKrw), payerName: issue.sale.payerName }
      : undefined,
  }
}
