import { describe, expect, it } from 'vitest'
import { formatNumber, formatPercent } from './format'

const usdSixDecimals: Intl.NumberFormatOptions = {
  style: 'currency',
  currency: 'USD',
  currencyDisplay: 'narrowSymbol',
  minimumFractionDigits: 6,
  maximumFractionDigits: 6,
}

describe('formatNumber', () => {
  it.each(['ko', 'en'] as const)('formats exact micro-dollar precision for %s', (locale) => {
    expect(formatNumber(0.000012, locale, usdSixDecimals)).toBe('$0.000012')
  })

  it('accepts fraction options instead of requiring call sites to round strings', () => {
    expect(formatNumber(12.6, 'en', { maximumFractionDigits: 0 })).toBe('13')
  })
})

describe('formatPercent', () => {
  it.each(['ko', 'en'] as const)(
    'formats ratios with locale-owned percent notation for %s',
    (locale) => {
      expect(formatPercent(1 / 3, locale)).toBe('33%')
    },
  )
})
