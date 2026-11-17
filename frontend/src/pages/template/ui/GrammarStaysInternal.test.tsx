import { describe, expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import type { FakeTemplateRow } from '@/test/templates'
import { openConversationDirect } from '@/test/authoring-ui'

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

/** The grammar is the contract between the template and the WRITE PROMPT. With templates made by
 *  talking, the one place it is visible is the secondary path that types the template's text
 *  directly (원문); the list, the template's own screen and its conversation keep it internal.
 *
 *  The assertions are on what is RENDERED rather than on the absence of a component, so a source
 *  view re-appearing anywhere else fails here. */
describe('the template grammar is visible in 원문 and nowhere else', () => {
  it('renders no grammar on the list, including its outlines', async () => {
    renderAppAt('/templates', { user: USER, templates: { templates: [EVERY_CONSTRUCT] } })
    await screen.findByRole('heading', { level: 1, name: '템플릿' })
    expect(await screen.findByRole('list', { name: '전부의 순서' })).toBeInTheDocument()
    expectNoGrammar()
  })

  it('renders no grammar on the template screen, its conversation or its preview', async () => {
    renderAppAt('/templates/template-all', {
      user: USER,
      templates: { templates: [EVERY_CONSTRUCT] },
    })
    const preview = within(await screen.findByRole('article', { name: '미리보기' }))
    expect(preview.getAllByText('사진 그룹마다 반복').length).toBeGreaterThan(0)
    expect(preview.getAllByText('네이버 지도').length).toBeGreaterThan(0)
    expect(screen.getByRole('log')).toBeInTheDocument()
    expectNoGrammar()
  })

  it('shows the body only inside 원문, the direct path', async () => {
    const user = userEvent.setup()
    renderAppAt('/templates/template-all', {
      user: USER,
      templates: { templates: [EVERY_CONSTRUCT] },
    })
    await screen.findByRole('log')
    expect(screen.queryByLabelText('원문')).not.toBeInTheDocument()
    await openConversationDirect(user)
    expect(await screen.findByLabelText('제목 원문')).toHaveValue(EVERY_CONSTRUCT.titleArea)
    expect(screen.getByLabelText('원문')).toHaveValue(EVERY_CONSTRUCT.body)
  })
})
