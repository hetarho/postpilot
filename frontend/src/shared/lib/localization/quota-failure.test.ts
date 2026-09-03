import { afterEach, describe, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { normalizeAppFailure } from '@/shared/api'
import { formatAppFailure } from './failure'

afterEach(() => initializeI18n('ko'))

// Change 19 A6: a credit refusal is rendered from its typed detail, never from the server's
// own message — and the machine values it carries (integer credits, an RFC3339 instant) are
// turned into the reader's own notation here, not guessed at by the server.
describe('credit refusals', () => {
  const refusal = () =>
    normalizeAppFailure({
      reason: 'INSUFFICIENT_CREDITS',
      params: { required: '79', balance: '12', renews_at: '2026-09-30T15:00:00Z' },
    })

  it('names what the work costs, what is left, and when it tops up, in Korean', () => {
    initializeI18n('ko')
    expect(formatAppFailure(refusal())).toBe(
      '크레딧이 79 필요한데 12만 남았어요. 2026. 10. 1. 오전 12:00에 충전돼요.',
    )
  })

  it('renders the same detail in English', () => {
    initializeI18n('en')
    expect(formatAppFailure(refusal(), 'en')).toBe(
      'This needs 79 credits and you have 12. Tops up Oct 1, 2026, 12:00 AM.',
    )
  })

  // A refusal that arrives without its numbers is not displayable: the boundary drops it to
  // the generic failure rather than rendering `{{required}}` at a user.
  it('refuses a credit detail that is missing a required param', () => {
    const partial = normalizeAppFailure({
      reason: 'INSUFFICIENT_CREDITS',
      params: { required: '79' },
    })
    expect(partial.reason).toBe('UNKNOWN_FAILURE')
  })
})
