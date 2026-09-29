import { describe, expect, it } from 'vitest'
import { create } from '@bufbuild/protobuf'
import {
  GetVoucherResponseSchema,
  ProtoPlan,
  VoucherPresetSchema,
  ProtoVoucherState,
} from '@/shared/api'
import { toGiftView, toVoucherPreset, toVoucherState } from './voucher-mappers'

describe('voucher mappers', () => {
  it('maps every state and reads one it has never heard of as unknown', () => {
    expect(toVoucherState(ProtoVoucherState.REDEEMABLE)).toBe('redeemable')
    expect(toVoucherState(ProtoVoucherState.REDEEMED)).toBe('redeemed')
    expect(toVoucherState(ProtoVoucherState.EXPIRED)).toBe('expired')
    expect(toVoucherState(ProtoVoucherState.REVOKED)).toBe('revoked')
    expect(toVoucherState(ProtoVoucherState.UNSPECIFIED)).toBe('unknown')
    expect(toVoucherState(99 as ProtoVoucherState)).toBe('unknown')
  })

  it('carries the public view and nothing else', () => {
    expect(
      toGiftView(
        create(GetVoucherResponseSchema, {
          credits: 330,
          validityDays: 30,
          message: 'hi',
          state: ProtoVoucherState.REDEEMABLE,
          linkExpiresAt: '2026-12-24T03:00:00Z',
        }),
      ),
    ).toEqual({
      credits: 330,
      validityDays: 30,
      message: 'hi',
      state: 'redeemable',
      linkExpiresAt: '2026-12-24T03:00:00Z',
    })
  })

  it('maps the four code-owned monthly bonus presets including Light', () => {
    for (const [plan, credits, name] of [
      [ProtoPlan.LIGHT, 290, 'light'],
      [ProtoPlan.BASIC, 510, 'basic'],
      [ProtoPlan.PRO, 1070, 'pro'],
      [ProtoPlan.MAX, 3170, 'max'],
    ] as const) {
      expect(
        toVoucherPreset(create(VoucherPresetSchema, { plan, credits, validityDays: 30 })),
      ).toEqual({ plan: name, credits, validityDays: 30 })
    }
  })
})
