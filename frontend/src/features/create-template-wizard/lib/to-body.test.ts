import { describe, expect, it } from 'vitest'
import { TEMPLATE_PARSE_OPTIONS, TEMPLATE_PHOTO_ROW_MAX, parseTemplate } from '@/entities/template'
import type { SectionDraft } from '../model/wizard-machine'
import { toWizardBody } from './to-body'

function section(patch: Partial<SectionDraft> = {}): SectionDraft {
  return {
    id: 'orderedMenu',
    role: 'orderedMenu',
    name: '주문한 메뉴',
    help: '주문한 메뉴의 모습과 맛을 씁니다',
    checked: true,
    heading: '먹어 본 메뉴',
    content: 'write',
    question: '주문한 메뉴',
    required: false,
    literal: '',
    photos: 0,
    repeatable: true,
    repeat: false,
    ...patch,
  }
}

const parses = (body: string) => parseTemplate('', body, TEMPLATE_PARSE_OPTIONS).ok

describe('the wizard body', () => {
  it('writes heading, content and photos in order for every content kind', () => {
    expect(toWizardBody([section()])).toBe(
      '먹어 본 메뉴\n<write>주문한 메뉴의 모습과 맛을 씁니다</write>',
    )
    // 직접 입력 asks the author and the AI writes the part from the answer (TMPL-74).
    expect(toWizardBody([section({ content: 'ask', required: true, photos: 2 })])).toBe(
      '먹어 본 메뉴\n<ask label="주문한 메뉴" required="true">주문한 메뉴의 모습과 맛을 씁니다</ask>\n<slot kind="photo" count="2"/>',
    )
    expect(toWizardBody([section({ content: 'literal', literal: '오늘의 메뉴', photos: 1 })])).toBe(
      '먹어 본 메뉴\n오늘의 메뉴\n<slot kind="photo"/>',
    )
  })

  it('wraps a repeating section in 사진마다 반복, but never a 직접 입력 field', () => {
    expect(toWizardBody([section({ photos: 1, repeat: true })])).toBe(
      '먹어 본 메뉴\n<repeat each="photo">\n<write>주문한 메뉴의 모습과 맛을 씁니다</write>\n<slot kind="photo"/>\n</repeat>',
    )
    const asking = toWizardBody([section({ content: 'ask', photos: 1, repeat: true })])
    expect(asking).not.toContain('<repeat')
    expect(parses(asking)).toBe(true)
    const parsed = parseTemplate('', asking, TEMPLATE_PARSE_OPTIONS)
    const field = parsed.ok ? parsed.nodes.find((node) => node.kind === 'ask') : undefined
    expect(field).toMatchObject({ label: '주문한 메뉴', text: '주문한 메뉴의 모습과 맛을 씁니다' })
  })

  it('builds bodies that parse for every content kind, repeat and photo count', () => {
    for (const content of ['write', 'ask', 'literal'] as const)
      for (let photos = 0; photos <= TEMPLATE_PHOTO_ROW_MAX; photos++)
        for (const repeat of [false, true]) {
          const body = toWizardBody([
            section({ content, photos, repeat, literal: '고정' }),
            section({
              id: 'verdict',
              heading: '총평 <요약>',
              question: '총평',
              content: 'write',
              help: '',
            }),
          ])
          expect(parses(body), `${content}/${photos}/${repeat}: ${body}`).toBe(true)
        }
  })

  it('leaves unchecked sections out and keeps a heading that looks like markup as text', () => {
    const body = toWizardBody([
      section({ checked: false }),
      section({ id: 'x', heading: '<b>굵게</b>' }),
    ])
    expect(body.startsWith('&lt;b>굵게&lt;/b>\n')).toBe(true)
    expect(parses(body)).toBe(true)
  })
})
