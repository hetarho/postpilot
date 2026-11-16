import { describe, expect, it } from 'vitest'
import { HEADING_REFERENCES_MAX, TEMPLATE_ASK_LABEL_MAX_CHARS } from '../config'
import { savedHeadingLines } from './heading-references'

describe('savedHeadingLines', () => {
  it('takes the short 고정 문구 lines of every body, repeats included, in directory order', () => {
    const lines = savedHeadingLines([
      {
        body: '📝 어쩌다 가게 됐냐면요...\n<write>방문 계기</write>\n<repeat each="photo">🍽️ 이건 꼭 드세요\n<write>메뉴</write>\n<slot kind="photo"/></repeat>\n',
      },
      { body: '  ☕ 공간 이야기  \n<write>공간</write>\n📝 어쩌다 가게 됐냐면요...\n' },
    ])
    expect(lines).toEqual(['📝 어쩌다 가게 됐냐면요...', '🍽️ 이건 꼭 드세요', '☕ 공간 이야기'])
  })

  it('skips write text, long fixed lines and bodies that do not parse, and decodes entities', () => {
    const long = '가'.repeat(TEMPLATE_ASK_LABEL_MAX_CHARS + 1)
    const lines = savedHeadingLines([
      { body: `<write>쓰기 지시는 소제목이 아니에요</write>\n${long}\n맛 &amp; 가격\n` },
      { body: '<unknown>깨진 본문</unknown>' },
    ])
    expect(lines).toEqual(['맛 & 가격'])
  })

  it('stops at the reference limit', () => {
    const body = Array.from({ length: HEADING_REFERENCES_MAX + 5 }, (_, i) => `소제목 ${i}`).join(
      '\n',
    )
    const lines = savedHeadingLines([{ body }, { body: '나중 템플릿 소제목' }])
    expect(lines).toHaveLength(HEADING_REFERENCES_MAX)
    expect(lines[0]).toBe('소제목 0')
    expect(lines).not.toContain('나중 템플릿 소제목')
  })

  it('gives none for no templates', () => {
    expect(savedHeadingLines([])).toEqual([])
  })
})
