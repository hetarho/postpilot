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
    body: '<write>사진마다 설명</write>',
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

const section = async (name: string) => within(await screen.findByRole('region', { name }))

describe('the template directory', () => {
  // Each template is shown as the shape it makes, one tile per template that opens it; the one
  // action is a conversation with the AI. Nothing that edits a template lives here.
  it('shows each template as a tile with its post count and offers only the AI conversation', async () => {
    const calls: string[] = []
    renderTemplates({}, calls)

    const list = await section('저장된 템플릿')
    const items = list.getAllByRole('listitem')
    expect(items).toHaveLength(2)
    expect(within(items[0]).getByRole('link', { name: /^일기/ })).toHaveAttribute(
      'href',
      '/templates/template-diary',
    )
    expect(within(items[1]).getByText('협찬 방문 리뷰')).toBeInTheDocument()
    expect(within(items[1]).getByText('글 2개')).toBeInTheDocument()

    // The create form, the block palette and the mode switch are all gone from this screen.
    expect(screen.queryByRole('group', { name: '블록 추가' })).not.toBeInTheDocument()
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('템플릿 이름')).not.toBeInTheDocument()
    // The add action floats over the list the way 새 글 floats over 글 — words only, no icon —
    // and opens the conversation that drafts a template from scratch.
    const create = screen.getByRole('link', { name: '새 템플릿' })
    expect(create).toHaveAttribute('href', '/templates/new')
    expect(create.closest('[data-action-bar="list"]')).not.toBeNull()
    expect(create.querySelector('svg')).toBeNull()
    // The kit's state in words stays on 홈: the header has no status line under its title.
    expect(within(screen.getByRole('main')).queryByRole('status')).not.toBeInTheDocument()
    // Deleting waits on the template's own screen, where it is being looked at.
    expect(within(items[1]).queryByRole('button')).not.toBeInTheDocument()

    // A12: the shell prepares model defaults; the list calls no AI and enqueues nothing ([I5]).
    const allowed = [
      'InitializeDefaultSelections',
      'GetMe',
      'GetMyPlan',
      'ListTemplates',
      'ListAuthoringSummaries',
      // The header draws the 템플릿 ring 홈 draws, from the same reads 홈 makes.
      'ListVoices',
      'ListGuidelines',
      'ListMemories',
      'ListModels',
      'GetSelections',
    ]
    expect(calls.filter((call) => !allowed.includes(call))).toEqual([])
  })

  // A1: the empty state says what a template is FOR. The grammar is what the write prompt reads,
  // never something shown to the user (A9).
  it('explains templates in plain language and shows no grammar when there are none', async () => {
    renderTemplates({ templates: [] })

    expect(
      await screen.findByRole('heading', { name: '어떤 글을 자주 올리세요?' }),
    ).toBeInTheDocument()
    expect(within(screen.getByRole('main')).queryByRole('listitem')).not.toBeInTheDocument()
    expect(screen.queryByRole('region', { name: '저장된 템플릿' })).not.toBeInTheDocument()
    const page = document.body.textContent ?? ''
    for (const syntax of ['<write', '<repeat', '<slot', '<note', '{작성}', '{반복}', '{자리}']) {
      expect(page).not.toContain(syntax)
    }
    // The conversation is still the way out of an empty gallery.
    expect(screen.getByRole('link', { name: '새 템플릿' })).toHaveAttribute(
      'href',
      '/templates/new',
    )
  })

  // A9: no grammar on the populated list either — a row shows the name and the description.
  it('renders no grammar syntax on a populated list', async () => {
    renderTemplates()
    await section('저장된 템플릿')
    const page = document.body.textContent ?? ''
    for (const syntax of ['<write', '<repeat', '<slot', '<note']) {
      expect(page).not.toContain(syntax)
    }
  })

  it('names how many posts lose a template before deleting it from its own screen', async () => {
    const user = userEvent.setup()
    const { router } = renderAppAt('/templates/template-review', {
      user: USER,
      templates: { templates: TEMPLATES },
    })

    await user.click(await screen.findByRole('button', { name: '정보성 식당 리뷰 삭제' }))
    const dialog = within(await screen.findByRole('dialog'))
    expect(dialog.getByText(/2개의 글에서 템플릿이 해제됩니다/)).toBeInTheDocument()
    await user.click(dialog.getByRole('button', { name: '삭제' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/templates'))
    await waitFor(() => expect(screen.queryByText('정보성 식당 리뷰')).not.toBeInTheDocument())
  })

  it('says so plainly when nothing references the template', async () => {
    const user = userEvent.setup()
    renderAppAt('/templates/template-diary', { user: USER, templates: { templates: TEMPLATES } })

    await user.click(await screen.findByRole('button', { name: '일기 삭제' }))
    const dialog = within(await screen.findByRole('dialog'))
    expect(dialog.getByText(/이 템플릿을 쓰는 글이 없어요/)).toBeInTheDocument()
  })

  // `post_count` is a projection over POSTS, so no template mutation can invalidate it.
  it('re-reads the directory on mount rather than serving a cached post count', async () => {
    const calls: string[] = []
    const { queryClient, transport } = renderTemplates({}, calls)
    await section('저장된 템플릿')
    await waitFor(() => expect(calls.filter((call) => call === 'ListTemplates')).toHaveLength(1))
    expect(queryClient.getQueryData(['templates', transport, USER.id])).toBeDefined()
  })

  it('offers a retry when the directory cannot be read', async () => {
    renderTemplates({ listFails: true })
    expect(await screen.findByText('템플릿 목록을 불러오지 못했어요.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '다시 시도' })).toBeInTheDocument()
  })
})
