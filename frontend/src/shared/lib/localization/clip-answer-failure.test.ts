import { afterEach, describe, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { normalizeAppFailure } from '@/shared/api'
import { formatAppFailure } from './failure'

afterEach(() => initializeI18n('ko'))

/** An answer stored before its template tightened its maximum is the one
 *  composition refusal an owner meets without ever opening the template body,
 *  so it has to say which answer and by how much (CLIP-102, CLIP-117). */
describe('bounded answer refusal', () => {
  const failure = normalizeAppFailure({
    reason: 'CLIP_COMPOSITION_INVALID',
    params: {
      element_id: 'place',
      line: '3',
      reason: 'answer_limit',
      label: '상호',
      max: '6',
      actual: '9',
    },
  })

  it.each(['ko', 'en'] as const)('names the field, its maximum and the count in %s', (locale) => {
    initializeI18n(locale)
    const message = formatAppFailure(failure, locale)
    expect(message).toContain('상호')
    expect(message).toContain('6')
    expect(message).toContain('9')
    expect(message).not.toContain('CLIP_COMPOSITION_INVALID')
    expect(message).not.toContain('3')
  })

  it('keeps the line-numbered sentence when no field is named', () => {
    const unlabelled = normalizeAppFailure({
      reason: 'CLIP_COMPOSITION_INVALID',
      params: { element_id: 'place', line: '3', reason: 'answer_limit' },
    })
    initializeI18n('ko')
    const message = formatAppFailure(unlabelled)
    expect(message).not.toBe('')
    expect(message).not.toContain('상호')
  })
})
