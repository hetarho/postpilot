import { afterEach, describe, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { normalizeAppFailure } from '@/shared/api'
import { formatAppFailure } from './failure'

afterEach(() => initializeI18n('ko'))

/** Every check the render-time verifier can fail reaches the owner as its OWN
 *  sentence, never as "invalid plan" (CDS-52, LANG-21) — which is what lets the
 *  correction screen say which rule stopped the render. */
describe('clip design-system refusals', () => {
  const layout = [
    'CLIP_LAYOUT_SAFE_AREA',
    'CLIP_LAYOUT_SIZE',
    'CLIP_LAYOUT_OVERLAP',
    'CLIP_LAYOUT_MOTION',
    'CLIP_LAYOUT_ANCHOR_STEP',
    'CLIP_LAYOUT_FREQUENCY',
    'CLIP_LAYOUT_DISCLOSURE',
    'CLIP_LAYOUT_KIND',
    'CLIP_DISCLOSURE_REQUIRED',
  ] as const

  it.each(layout)('formats %s as its own message in both locales', (reason) => {
    const failure = normalizeAppFailure({ reason, params: {} })
    const seen = new Set<string>()
    for (const locale of ['ko', 'en'] as const) {
      initializeI18n(locale)
      const message = formatAppFailure(failure, locale)
      expect(message).not.toBe('')
      expect(message).not.toContain(reason)
      expect(message).not.toContain('UNKNOWN')
      seen.add(message)
    }
    expect(seen.size).toBe(2)
  })

  it('names the labels a clip is still missing', () => {
    const failure = normalizeAppFailure({
      reason: 'CLIP_FACTS_REQUIRED',
      params: { labels: '상호, 가격' },
    })
    initializeI18n('ko')
    expect(formatAppFailure(failure)).toContain('상호, 가격')
    initializeI18n('en')
    expect(formatAppFailure(failure, 'en')).toContain('상호, 가격')
  })
})
