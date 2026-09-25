import {
  ProtoVoucherState,
  type GetVoucherResponse,
  type RedeemVoucherResponse,
} from '@/shared/api'
import type { GiftView, Redemption, VoucherState } from '../model/types'

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
