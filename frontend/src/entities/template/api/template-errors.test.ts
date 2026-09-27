import { Code } from '@connectrpc/connect'
import { afterEach, describe, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { connectAppError } from '@/test/app-error'
import { templateErrorMessage } from './template-errors'

afterEach(() => initializeI18n('ko'))

// TMPL-20, TMPL-42: a server parse refusal reads as the builder's own sentence for its reason and
// names the area it sits in, never the raw key; TMPL-6: a number refusal states its whole range.
describe('template refusals', () => {
  it('says a parse reason in the builder words and names its area', () => {
    const title = connectAppError('TEMPLATE_PARSE_FAILED', Code.InvalidArgument, {
      line: '3',
      reason: 'malformed_tag',
      area: 'title_area',
    })
    initializeI18n('ko')
    expect(templateErrorMessage(title)).toBe(
      '제목 3번째 줄: 표기 형식이 잘못됐어요. 원문에서 고쳐 주세요.',
    )
    initializeI18n('en')
    expect(templateErrorMessage(title)).toBe(
      'Title line 3: malformed notation. Fix it in the source view.',
    )
  })

  it('reads a refusal without an area as the body, with its ceiling filled in', () => {
    const body = connectAppError('TEMPLATE_PARSE_FAILED', Code.InvalidArgument, {
      line: '2',
      reason: 'invalid_count',
    })
    initializeI18n('ko')
    const message = templateErrorMessage(body)
    expect(message).toMatch(/^본문 2번째 줄: /)
    expect(message).not.toContain('invalid_count')
    expect(message).not.toContain('{{')
  })

  it('states the range a template number missed', () => {
    const refusal = connectAppError('TEMPLATE_NUMBER_OUT_OF_RANGE', Code.InvalidArgument, {
      field: 'target_length',
      min: '100',
      max: '10000',
      actual: '12000',
    })
    initializeI18n('ko')
    expect(templateErrorMessage(refusal)).toBe('100~10000 사이로 적어 주세요. 지금은 12000이에요.')
    initializeI18n('en')
    expect(templateErrorMessage(refusal)).toBe(
      'The value must be between 100 and 10000. It is currently 12000.',
    )
  })
})
