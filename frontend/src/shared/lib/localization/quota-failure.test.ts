import { afterEach, describe, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { normalizeAppFailure } from '@/shared/api'
import { formatAppFailure } from './failure'

afterEach(() => initializeI18n('ko'))

// Job 37 A9: a quota refusal is rendered from its typed detail, never from the server's own
// message — and the machine values it carries (micro-USD, an RFC3339 instant, a tier name) are
// turned into the reader's own notation here, not guessed at by the server.
describe('plan refusals', () => {
  const quota = (reason: 'DAILY_COUNT' | 'DAILY_COST' | 'MONTHLY_COST') =>
    normalizeAppFailure({
      reason,
      params: { limit: '100000', used: '100000', resets_at: '2026-09-01T15:00:00Z' },
    })

  it('names the axis, the limit and the reset instant in Korean', () => {
    initializeI18n('ko')
    expect(formatAppFailure(quota('DAILY_COST'))).toBe(
      '오늘 사용할 수 있는 금액 US$0.10를 모두 썼어요. 2026. 9. 2. 오전 12:00에 다시 열려요.',
    )
    expect(formatAppFailure(quota('DAILY_COUNT'))).toContain('AI 작업 100000회')
    expect(formatAppFailure(quota('MONTHLY_COST'))).toContain('이번 달')
  })

  it('renders the same detail in English', () => {
    initializeI18n('en')
    expect(formatAppFailure(quota('MONTHLY_COST'), 'en')).toBe(
      "You have used this month's $0.10 budget. It reopens at Sep 2, 2026, 12:00 AM.",
    )
  })

  it('names the tier a locked model needs, the way the badge does', () => {
    initializeI18n('ko')
    const locked = normalizeAppFailure({
      reason: 'MODEL_LOCKED',
      params: {
        model: 'openrouter/anthropic/claude-opus-5',
        models: 'openrouter/anthropic/claude-opus-5',
        required_plan: 'max',
      },
    })
    expect(formatAppFailure(locked)).toBe(
      'openrouter/anthropic/claude-opus-5 모델은 Max 플랜부터 쓸 수 있어요.',
    )
  })

  // An axis that arrives without its numbers is not displayable: the boundary drops it to the
  // generic failure rather than rendering `{{limit}}` at a user.
  it('refuses a quota detail that is missing a required param', () => {
    const partial = normalizeAppFailure({ reason: 'DAILY_COUNT', params: { limit: '10' } })
    expect(partial.reason).toBe('UNKNOWN_FAILURE')
  })
})
