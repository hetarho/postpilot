import { describe, expect, it } from 'vitest'
import { parse } from '../lib/grammar'
import { readCompositionWorkingState, toWorkingBody, type BuilderBlock } from './blocks'
import { TEMPLATE_PARSE_OPTIONS } from './types'
describe('unpublished builder rows', () => {
  it.each(['text', 'write'] as const)(
    'retains an empty %s row as invalid source plus exact private view state',
    (kind) => {
      const blocks: BuilderBlock[] = [
        { id: 'saved', kind: 'write', text: '내용' },
        { id: 'new', kind, text: '' },
      ]
      const source = toWorkingBody(blocks)
      expect(source).toBe('<write>내용</write>\n<write></write>')
      expect(parse(source, TEMPLATE_PARSE_OPTIONS).ok).toBe(false)
      expect(readCompositionWorkingState({ source, blocks }, source)?.blocks).toEqual(blocks)
    },
  )
  it('keeps duplicate and unfinished ask rows rather than omitting them from a saveable body', () => {
    const blocks: BuilderBlock[] = [
      { id: 'a', kind: 'write', text: '메뉴', ask: '질문' },
      { id: 'b', kind: 'text', text: 'retained', ask: '질문' },
      { id: 'c', kind: 'text', text: '숨겨진 원문', ask: '' },
    ]
    const source = toWorkingBody(blocks)
    expect(source).toContain('<ask label="질문"/>')
    expect(source).toContain('<ask label=""/>')
    expect(parse(source, TEMPLATE_PARSE_OPTIONS).ok).toBe(false)
    expect(readCompositionWorkingState({ source, blocks }, source)?.blocks[2]).toMatchObject({
      text: '숨겨진 원문',
      ask: '',
    })
  })
  it('rejects stale, mismatched and malformed metadata so a raw edit stays authoritative', () => {
    const blocks: BuilderBlock[] = [{ id: 'a', kind: 'write', text: '메뉴' }]
    expect(readCompositionWorkingState({ source: 'old', blocks }, 'new')).toBeUndefined()
    expect(
      readCompositionWorkingState(
        { source: '<write>다른 내용</write>', blocks },
        '<write>다른 내용</write>',
      ),
    ).toBeUndefined()
    expect(
      readCompositionWorkingState({ source: '', blocks: [{ id: 'x', kind: 'unknown' }] }, ''),
    ).toBeUndefined()
  })
})
