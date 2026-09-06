import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import type { FakeTemplateRow } from '@/test/templates'

const USER = { id: 'alice' }

/** Every construct the grammar has, so no single one can slip through unnoticed. Two of them —
 *  the place and link positions — are retired and appear only in stored bodies, which is exactly
 *  why the fixture keeps them. */
const EVERY_CONSTRUCT: FakeTemplateRow = {
  id: 'template-all',
  name: '전부',
  description: '모든 구성요소',
  body:
    '<write>인트로를 씁니다</write>\n머리말 그대로\n<slot kind="place" label="네이버 지도"/>\n' +
    '<repeat each="photo">\n<slot kind="photo" count="3"/>\n<write>이 사진에 대한 설명</write>\n</repeat>\n' +
    '<slot kind="link" label="예약"/>\n<note>광고 티 내지 말 것</note>',
}

/** The tag syntax, the attribute names, and the brace notation the old empty state invented. */
const SYNTAX = [
  '<write',
  '</write',
  '<repeat',
  '</repeat',
  '<slot',
  '<note',
  '</note',
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

/** The grammar is the contract between the builder and the WRITE PROMPT, and r3 gives it exactly
 *  ONE place to be visible: 원문. Everywhere else — the list, the builder with every row open, the
 *  unreadable state before the user asks to fix it — it stays internal (TEMPLATE-26, TEMPLATE-30).
 *
 *  The assertions are on what is RENDERED rather than on the absence of a component, so a source
 *  view re-appearing anywhere else fails here. */
describe('the template grammar is visible in 원문 and nowhere else', () => {
  it('renders no grammar on the list, including its empty state', async () => {
    renderAppAt('/templates', { user: USER, templates: { templates: [EVERY_CONSTRUCT] } })
    await screen.findByRole('heading', { level: 1, name: '템플릿' })
    expectNoGrammar()
  })

  it('renders no grammar in the builder, with every construct and every row open', async () => {
    const user = userEvent.setup()
    renderAppAt('/templates/template-all', {
      user: USER,
      templates: { templates: [EVERY_CONSTRUCT] },
    })

    await screen.findByLabelText('이름')
    expectNoGrammar()

    // Nor while a block of each kind is open for editing: a field shows the block's own text,
    // never the tags that carry it.
    const toggles = screen
      .getAllByRole('button')
      .filter((button) => button.getAttribute('aria-expanded') !== null)
    for (const control of toggles) {
      await user.click(control)
      expectNoGrammar()
    }
  })

  it('renders no grammar on a body it cannot parse until the user asks to fix it', async () => {
    renderAppAt('/templates/template-broken', {
      user: USER,
      templates: {
        templates: [{ id: 'template-broken', name: '옛 템플릿', body: '<write>닫히지 않음' }],
      },
    })

    expect(await screen.findByText(/구성을 읽을 수 없어요/)).toBeInTheDocument()
    // The state that OFFERS the source view still shows none of it.
    expectNoGrammar()
  })

  // The r3 rule, both halves: the switch exists, and choosing 원문 is the one thing that makes the
  // body visible as text.
  it('offers the mode switch and shows the body only inside 원문', async () => {
    const user = userEvent.setup()
    renderAppAt('/templates/template-all', {
      user: USER,
      templates: { templates: [EVERY_CONSTRUCT] },
    })
    await screen.findByLabelText('이름')

    expect(screen.getByRole('tablist', { name: '구성 편집 방식' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: '블록' })).toBeInTheDocument()

    await user.click(screen.getByRole('tab', { name: '원문' }))
    expect(screen.getByLabelText('원문')).toHaveValue(EVERY_CONSTRUCT.body)
    // And the guide that teaches the same grammar is here too, and only here.
    expect(screen.getByText('형식 안내 보기')).toBeInTheDocument()

    await user.click(screen.getByRole('tab', { name: '블록' }))
    expect(screen.queryByLabelText('원문')).not.toBeInTheDocument()
    expect(screen.queryByText('형식 안내 보기')).not.toBeInTheDocument()
    expectNoGrammar()
  })
})
