import { describe, expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import { openTemplateDirect } from '@/test/authoring-ui'
import type { FakeTemplateRow } from '@/test/templates'

const USER = { id: 'alice' }

/** Every construct the grammar has, so no single one can slip through unnoticed. Two of them —
 *  the place and link positions — are retired and appear only in stored bodies, which is exactly
 *  why the fixture keeps them. The title area carries the constructs it admits (TMPL-50). */
const EVERY_CONSTRUCT: FakeTemplateRow = {
  id: 'template-all',
  name: '전부',
  description: '모든 구성요소',
  titleArea: '<ask label="가게 이름"/> 방문 후기 <write>메뉴를 한 줄로</write>',
  body:
    '<write>인트로를 씁니다</write>\n머리말 그대로\n<slot kind="place" label="네이버 지도"/>\n' +
    '<repeat each="photo">\n<slot kind="photo" count="3"/>\n<write>이 사진에 대한 설명</write>\n</repeat>\n' +
    '<slot kind="link" label="예약"/>',
}

/** The tag syntax, the attribute names, and the brace notation the old empty state invented. */
const SYNTAX = [
  '<write',
  '</write',
  '<ask',
  '</ask',
  '<repeat',
  '</repeat',
  '<slot',
  'each="photo"',
  'kind="place"',
  'kind="photo"',
  'kind="link"',
  'label=',
  'count="',
  '{작성}',
  '{자리}',
  '{반복}',
  '{사진}',
]

function expectNoGrammar() {
  const rendered = document.body.textContent ?? ''
  for (const syntax of SYNTAX) {
    expect(rendered).not.toContain(syntax)
  }
}

/** The grammar is the contract between the template and the WRITE PROMPT, and it is visible in ONE
 *  place: 원문, where the owner edits the text by hand. Everywhere else — the list's post shapes,
 *  the conversation and its live draft — it stays internal (TMPL-26, TMPL-65).
 *
 *  The assertions are on what is RENDERED rather than on the absence of a component, so a source
 *  view re-appearing anywhere else fails here. */
describe('the template grammar is visible in 원문 and nowhere else', () => {
  it('renders no grammar on the list, including its post shapes', async () => {
    renderAppAt('/templates', { user: USER, templates: { templates: [EVERY_CONSTRUCT] } })
    await screen.findByRole('heading', { level: 1, name: '템플릿' })
    await screen.findByRole('link', { name: '전부' })
    expectNoGrammar()
  })

  // The draft beside the conversation draws every construct as its stand-in and never as the
  // tags that carry it (TMPL-65, TMPL-66).
  it('renders no grammar in the conversation or its live draft', async () => {
    renderAppAt('/templates/template-all', {
      user: USER,
      templates: { templates: [EVERY_CONSTRUCT] },
    })
    await screen.findByRole('log', { name: '템플릿 대화' })
    const draft = within(screen.getByRole('complementary', { name: '초안' }))
    expect((await draft.findAllByText('사진 그룹마다 반복')).length).toBeGreaterThan(0)
    expect(draft.getAllByText('네이버 지도').length).toBeGreaterThan(0)
    expectNoGrammar()
  })

  it('shows the grammar only as 원문, with its format guide, when the owner edits by hand', async () => {
    const user = userEvent.setup()
    renderAppAt('/templates/template-all', {
      user: USER,
      templates: { templates: [EVERY_CONSTRUCT] },
    })
    await screen.findByRole('log', { name: '템플릿 대화' })
    await openTemplateDirect(user)
    expect(screen.getByLabelText('제목 원문')).toHaveValue(EVERY_CONSTRUCT.titleArea)
    expect(screen.getByLabelText('원문')).toHaveValue(EVERY_CONSTRUCT.body)
    expect(screen.getByText('형식 안내 보기')).toBeInTheDocument()
    expect(screen.queryByRole('tablist', { name: '구성 편집 방식' })).not.toBeInTheDocument()
  })
})
