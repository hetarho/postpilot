import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import type { FakeTemplateRow } from '@/test/templates'

const USER = { id: 'alice' }

/** Every construct the grammar has, so no single one can slip through unnoticed. */
const EVERY_CONSTRUCT: FakeTemplateRow = {
  id: 'template-all',
  name: '전부',
  description: '모든 구성요소',
  body:
    '<write>인트로를 씁니다</write>\n머리말 그대로\n<slot kind="place" label="네이버 지도"/>\n' +
    '<repeat each="photo">\n<slot kind="photo"/>\n<write>이 사진에 대한 설명</write>\n</repeat>\n' +
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

/** The grammar is the contract between the builder and the WRITE PROMPT. It is an internal
 *  encoding, and change 30 A9 makes that a rule the screens are held to rather than a habit:
 *  assert on what is rendered, not on the absence of a component, so re-introducing a source
 *  view anywhere fails here. */
describe('the template grammar never reaches the user', () => {
  it('renders no grammar on the list, including its empty state', async () => {
    renderAppAt('/templates', { user: USER, templates: { templates: [EVERY_CONSTRUCT] } })
    await screen.findByRole('heading', { level: 1, name: '템플릿' })
    expectNoGrammar()
  })

  it('renders no grammar on a template with every construct in it', async () => {
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

  it('renders no grammar on a body it cannot even parse', async () => {
    renderAppAt('/templates/template-broken', {
      user: USER,
      templates: {
        templates: [{ id: 'template-broken', name: '옛 템플릿', body: '<write>닫히지 않음' }],
      },
    })

    // The one state that used to force the source view open is now the one state with no source
    // view at all.
    expect(await screen.findByText(/구성을 읽을 수 없어요/)).toBeInTheDocument()
    expectNoGrammar()
  })

  it('offers no mode switch anywhere', async () => {
    renderAppAt('/templates/template-all', {
      user: USER,
      templates: { templates: [EVERY_CONSTRUCT] },
    })
    await screen.findByLabelText('이름')
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument()
    expect(screen.queryByRole('tab', { name: '원문' })).not.toBeInTheDocument()
  })
})
