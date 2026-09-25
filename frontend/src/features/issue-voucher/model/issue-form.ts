import { VOUCHER_LIMITS, type VoucherIssue, type VoucherPreset } from '@/entities/voucher'

export type IssueMode = VoucherPreset['plan'] | 'custom'

export interface IssueDraft {
  mode: IssueMode
  credits: string
  days: string
  kind: 'sold' | 'given'
  amount: string
  payer: string
  message: string
}

export type IssueError = 'credits' | 'days' | 'amount' | 'payer' | 'message'

/** Digits only: the amount and the custom numbers are typed on a phone keypad, and separators
 *  the display adds must not come back as input. */
export function digitsOnly(value: string): string {
  return value.replace(/\D/g, '')
}

function inRange(value: string, max: number): number | undefined {
  if (value === '') return undefined
  const parsed = Number(value)
  return Number.isInteger(parsed) && parsed >= 1 && parsed <= max ? parsed : undefined
}

/** Turns the form into an issue, or names every field outside the product's bounds. */
export function toIssue(
  draft: IssueDraft,
  presets: readonly VoucherPreset[],
): { issue: VoucherIssue } | { errors: IssueError[] } {
  const errors: IssueError[] = []
  const preset = presets.find((candidate) => candidate.plan === draft.mode)
  const credits = preset ? preset.credits : inRange(draft.credits, VOUCHER_LIMITS.maxCredits)
  const days = preset ? preset.validityDays : inRange(draft.days, VOUCHER_LIMITS.maxDays)
  if (credits === undefined) errors.push('credits')
  if (days === undefined) errors.push('days')
  const message = draft.message.trim()
  if ([...message].length > VOUCHER_LIMITS.maxMessage) errors.push('message')

  let sale: VoucherIssue['sale']
  if (draft.kind === 'sold') {
    const amount = inRange(draft.amount, VOUCHER_LIMITS.maxSaleKrw)
    const payer = draft.payer.trim()
    if (amount === undefined) errors.push('amount')
    if (payer === '' || [...payer].length > VOUCHER_LIMITS.maxPayer) errors.push('payer')
    if (amount !== undefined) sale = { amountKrw: amount, payerName: payer }
  }
  if (errors.length > 0 || credits === undefined || days === undefined) return { errors }
  return { issue: { credits, validityDays: days, message, sale } }
}
