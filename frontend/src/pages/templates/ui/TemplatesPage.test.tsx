import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import type { FakeTemplateRow, FakeTemplatesOptions } from '@/test/templates'

const USER = { id: 'alice' }

const TEMPLATES: FakeTemplateRow[] = [
  {
    id: 'template-review',
    name: '정보성 식당 리뷰',
    description: '협찬 방문 리뷰',
    body: '어쩌다 가게 됐냐면요\n<slot kind="photo" count="2"/>\n<write>사진마다 설명</write>\n<ask label="메뉴와 가격"/>',
    postCount: 2,
  },
  { id: 'template-diary', name: '일기', body: '<write>그날 있었던 일</write>' },
]

function renderTemplates(templates: FakeTemplatesOptions = {}, calls: string[] = []) {
  return renderAppAt('/templates', {
    user: USER,
    calls,
    templates: { templates: TEMPLATES, ...templates },
  })
}

const WRITES = /^(Start|Create|Generate|Analyze|Save|Update|Delete|Estimate)/

describe('the template directory', () => {
  // Each template is drawn as the outline of the post it makes, so the list is read as shapes.
  it('draws each template as the outline of its post and offers only the add action', async () => {
    const calls: string[] = []
    renderTemplates({}, calls)

    const list = within(await screen.findByRole('region', { name: '저장된 템플릿' }))
    const items = list
      .getAllByRole('listitem', { name: undefined })
      .filter((item) => item.querySelector('a[href^="/templates/"]'))
    expect(items).toHaveLength(2)
    expect(list.getByRole('link', { name: '일기' })).toHaveAttribute(
      'href',
      '/templates/template-diary',
    )
    const outline = list.getByRole('list', { name: '정보성 식당 리뷰의 순서' })
    expect(
      within(outline)
        .getAllByRole('listitem')
        .map((item) => item.textContent),
    ).toEqual(['어쩌다 가게 됐냐면요', '사진 2장', 'AI가 쓰는 글', '물어볼 것 · 메뉴와 가격'])
    expect(list.getByText('글 2개')).toBeInTheDocument()

    // Nothing that edits or deletes a template lives here; a card is the way into it.
    expect(screen.queryByRole('button', { name: /삭제/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('group', { name: '블록 추가' })).not.toBeInTheDocument()
    // The add action is the section's one creation button, straight to the conversation.
    expect(screen.getByRole('link', { name: '새 템플릿' })).toHaveAttribute(
      'href',
      '/templates/new',
    )
    // Reads only: the list calls no AI and enqueues nothing ([I5]).
    await waitFor(() => expect(calls).toContain('ListAuthoringSummaries'))
    expect(calls.filter((call) => WRITES.test(call))).toEqual([])
  })

  it('opens on the empty sheet a first conversation fills, with no grammar', async () => {
    renderTemplates({ templates: [] })

    expect(await screen.findByText('아직 저장된 템플릿이 없어요')).toBeInTheDocument()
    expect(screen.queryByRole('region', { name: '저장된 템플릿' })).not.toBeInTheDocument()
    const page = document.body.textContent ?? ''
    for (const syntax of ['<write', '<repeat', '<slot', '<ask', '{작성}', '{반복}', '{자리}']) {
      expect(page).not.toContain(syntax)
    }
    expect(screen.getByRole('link', { name: '새 템플릿' })).toBeInTheDocument()
  })

  // Deleting moved onto the template's own screen; the sheet still says how many posts lose it.
  it('deletes from the template screen, naming how many posts lose their template', async () => {
    const user = userEvent.setup()
    const { router } = renderAppAt('/templates/template-review', {
      user: USER,
      templates: { templates: TEMPLATES },
    })
    // It is a row of the conversation's ⋯, in the screen's one title row.
    await user.click(await screen.findByRole('button', { name: '더 보기' }))
    await user.click(await screen.findByRole('menuitem', { name: '템플릿 삭제' }))
    const dialog = within(await screen.findByRole('dialog'))
    expect(dialog.getByText(/2개의 글에서 템플릿이 해제됩니다/)).toBeInTheDocument()
    await user.click(dialog.getByRole('button', { name: '삭제' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/templates'))
    await waitFor(() => expect(screen.queryByText('정보성 식당 리뷰')).not.toBeInTheDocument())
  })

  // `post_count` is a projection over POSTS, so no template mutation can invalidate it.
  it('re-reads the directory on mount rather than serving a cached post count', async () => {
    const calls: string[] = []
    const { queryClient, transport } = renderTemplates({}, calls)
    await screen.findByRole('region', { name: '저장된 템플릿' })
    await waitFor(() => expect(calls.filter((call) => call === 'ListTemplates')).toHaveLength(1))
    expect(queryClient.getQueryData(['templates', transport, USER.id])).toBeDefined()
  })

  it('offers a retry when the directory cannot be read', async () => {
    renderTemplates({ listFails: true })
    expect(await screen.findByText('템플릿 목록을 불러오지 못했어요.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '다시 시도' })).toBeInTheDocument()
  })
})
