import { afterEach, describe, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { normalizeAppFailure } from '@/shared/api'
import { formatAppFailure } from './failure'

afterEach(() => initializeI18n('ko'))

/** A narration caption is written by the model, not by a template line, so its
 *  refusal carries line 0. The owner is told which caption, never "line 0". */
describe('narration caption refusal', () => {
  const failure = normalizeAppFailure({
    reason: 'CLIP_COMPOSITION_INVALID',
    params: { element_id: 'narration-2', line: '0', reason: 'copy_limit' },
  })

  it('names the caption by its place in the clip', () => {
    initializeI18n('ko')
    expect(formatAppFailure(failure)).toBe('영상의 2번째 자막을 확인해 주세요.')
  })

  it('names the caption in en', () => {
    initializeI18n('en')
    expect(formatAppFailure(failure, 'en')).toBe('Check caption 2 in the video.')
  })

  it('never shows a line 0 for another element without a line', () => {
    const lineless = normalizeAppFailure({
      reason: 'CLIP_COMPOSITION_INVALID',
      params: { element_id: 'legacy-hook', line: '0', reason: 'copy_limit' },
    })
    for (const locale of ['ko', 'en'] as const) {
      initializeI18n(locale)
      const message = formatAppFailure(lineless, locale)
      expect(message).toContain('legacy-hook')
      expect(message).not.toMatch(/\b0\b|0번째/)
    }
  })

  it('keeps the line-numbered sentence for a template line', () => {
    const lined = normalizeAppFailure({
      reason: 'CLIP_COMPOSITION_INVALID',
      params: { element_id: 'intro', line: '15', reason: 'copy_limit' },
    })
    initializeI18n('ko')
    expect(formatAppFailure(lined)).toBe('영상 구성의 15번째 줄(intro)을 확인해 주세요.')
  })
})
