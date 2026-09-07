import { afterEach, describe, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { normalizeAppFailure } from '@/shared/api'
import { formatAppFailure } from './failure'

afterEach(() => initializeI18n('ko'))

describe('authentication throttle refusal', () => {
  const refusal = () =>
    normalizeAppFailure({
      reason: 'TOO_MANY_ATTEMPTS',
      params: { retry_at: '2026-09-30T15:00:00Z' },
    })

  it('formats the retry instant in Korean', () => {
    initializeI18n('ko')
    expect(formatAppFailure(refusal())).toBe(
      '요청이 너무 많아요. 2026. 10. 1. 오전 12:00 이후 다시 시도해 주세요.',
    )
  })

  it('formats the retry instant in English', () => {
    initializeI18n('en')
    expect(formatAppFailure(refusal(), 'en')).toBe(
      'Too many requests. Try again after Oct 1, 2026, 12:00 AM.',
    )
  })
})
