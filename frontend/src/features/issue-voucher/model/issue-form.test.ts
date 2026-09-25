import { describe, expect, it } from 'vitest'
import type { VoucherPreset } from '@/entities/voucher'
import { digitsOnly, toIssue, type IssueDraft } from './issue-form'

const PRESETS: VoucherPreset[] = [
  { plan: 'basic', credits: 330, validityDays: 30 },
  { plan: 'pro', credits: 1150, validityDays: 30 },
  { plan: 'max', credits: 2400, validityDays: 30 },
]

const draft = (patch: Partial<IssueDraft>): IssueDraft => ({
  mode: 'pro',
  credits: '',
  days: '',
  kind: 'given',
  amount: '',
  payer: '',
  message: '',
  ...patch,
})

describe('toIssue', () => {
  it('takes a preset as it is and trims the message', () => {
    expect(toIssue(draft({ mode: 'basic', message: '  감사합니다 ' }), PRESETS)).toEqual({
      issue: { credits: 330, validityDays: 30, message: '감사합니다', sale: undefined },
    })
  })

  it('reads custom numbers and a sale', () => {
    expect(
      toIssue(
        draft({
          mode: 'custom',
          credits: '500',
          days: '14',
          kind: 'sold',
          amount: '9900',
          payer: ' 김민수 ',
        }),
        PRESETS,
      ),
    ).toEqual({
      issue: {
        credits: 500,
        validityDays: 14,
        message: '',
        sale: { amountKrw: 9900, payerName: '김민수' },
      },
    })
  })

  it('names every field outside the bounds', () => {
    expect(
      toIssue(
        draft({
          mode: 'custom',
          credits: '100001',
          days: '0',
          kind: 'sold',
          amount: '',
          payer: '가'.repeat(41),
          message: '가'.repeat(81),
        }),
        PRESETS,
      ),
    ).toEqual({ errors: ['credits', 'days', 'message', 'amount', 'payer'] })
  })

  it('keeps separators out of typed numbers', () => {
    expect(digitsOnly('14,900원')).toBe('14900')
  })
})
