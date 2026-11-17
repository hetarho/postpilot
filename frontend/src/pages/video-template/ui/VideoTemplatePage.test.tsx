import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import { openConversationDirect, saveConversationDraft } from '@/test/authoring-ui'
import type { FakeClipsOptions } from '@/test/clips'

const template = {
  id: 'owned',
  name: '여행',
  compositionBody: '<clip version="1"/>',
  projectCount: 2,
}
const mount = (path: string, clips: FakeClipsOptions = {}) =>
  renderAppAt(path, { user: { id: 'alice' }, clips: { templates: [template], ...clips } })

describe('video template workflow', () => {
  it('renders empty and failed directories the way the post templates do', async () => {
    const empty = mount('/video-templates', { templates: [] })
    // Two parts, like `/templates`: what is missing, and how one is made — by talking.
    expect(
      await screen.findByRole('heading', { name: '아직 저장된 영상 템플릿이 없어요' }),
    ).toBeInTheDocument()
    expect(screen.getByText(/어떤 영상을 올리는지 AI에게 말하면/)).toBeInTheDocument()
    empty.unmount()
    mount('/video-templates', { listFails: true })
    expect(await screen.findByRole('alert')).toHaveTextContent('영상 템플릿을 불러오지 못했어요.')
    expect(screen.getByRole('button', { name: '다시 시도' })).toBeInTheDocument()
  })
  it('badges how many clips each template is used by, and docks its creation button', async () => {
    mount('/video-templates')
    const region = within(await screen.findByRole('region', { name: '저장된 영상 템플릿' }))
    expect(region.getByText('클립 2개')).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: '새 영상 템플릿' })).toBeNull()
    expect(screen.getByRole('link', { name: '새 템플릿' })).toHaveAttribute(
      'href',
      '/video-templates/new',
    )
  })
  it("heads the detail with the template's own name, and the create screen with its own", async () => {
    const editing = mount('/video-templates/owned')
    expect(await screen.findByRole('heading', { name: '여행', level: 1 })).toBeInTheDocument()
    editing.unmount()
    mount('/video-templates/new')
    expect(await screen.findByRole('heading', { name: '새 영상 템플릿' })).toBeInTheDocument()
  })
  it('keeps the structural back link when a new clip’s offer opened the creation', async () => {
    // As a post template opened with `post=new` does (TMPL-32); 저장 is what returns to the clip.
    mount('/video-templates/new?clip=new')
    expect(await screen.findByRole('link', { name: '영상 템플릿' })).toHaveAttribute(
      'href',
      '/video-templates',
    )
    expect(
      screen
        .getAllByRole('link')
        .filter((link) => link.getAttribute('href') === '/video-templates'),
    ).toHaveLength(1)
  })
  it('requires a session before requesting templates', async () => {
    const calls: string[] = []
    const { router } = renderAppAt('/video-templates', { calls })
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    expect(calls).not.toContain('ListVideoTemplates')
  })
  it('lists metadata and opens an authenticated lazy editor', async () => {
    const user = userEvent.setup()
    const { router } = mount('/video-templates')
    const region = within(await screen.findByRole('region', { name: '저장된 영상 템플릿' }))
    await user.click(await region.findByRole('link', { name: /여행/ }))
    // The template opens on itself with the conversation beside it; its text is one ⋯ away.
    expect(await screen.findByRole('textbox', { name: 'AI에게 보낼 말' })).toBeInTheDocument()
    await openConversationDirect(user)
    expect(await screen.findByLabelText('템플릿 이름')).toHaveValue('여행')
    expect(router.state.location.pathname).toBe('/video-templates/owned')
    expect(screen.queryByRole('tab', { name: '원문' })).not.toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('keeps edits after a localized server refusal', async () => {
    const user = userEvent.setup()
    mount('/video-templates/owned', { saveFails: true })
    await openConversationDirect(user)
    const name = await screen.findByLabelText('템플릿 이름')
    await user.clear(name)
    await user.type(name, '실패해도 보존')
    await saveConversationDraft(user)
    expect(await screen.findByText('같은 이름의 영상 템플릿이 이미 있어요.')).toBeInTheDocument()
  })
  it.each(['unknown', 'foreign'])(
    'keeps %s indistinguishable from a missing template',
    async (id) => {
      mount(`/video-templates/${id}`, {
        templates: [{ ...template, id: 'foreign', ownerId: 'bob' }],
      })
      expect(await screen.findByText('영상 템플릿을 찾을 수 없어요.')).toBeInTheDocument()
      expect(screen.queryByLabelText('템플릿 이름')).not.toBeInTheDocument()
    },
  )
  // The delete lives on the template's own screen, as a row of its ⋯ (as for a post template),
  // and the detach is warned about BEFORE the delete.
  it('warns how many clips a delete detaches, then returns to the list without it', async () => {
    const user = userEvent.setup()
    const { router, queryClient, transport } = mount('/video-templates/owned')
    const projectKey = ['clip-projects', transport, 'alice', 'list']
    const otherOwnerKey = ['clip-projects', transport, 'bob', 'list']
    queryClient.setQueryData(projectKey, [])
    queryClient.setQueryData(otherOwnerKey, [])
    await user.click(await screen.findByRole('button', { name: '더 보기' }))
    await user.click(await screen.findByRole('menuitem', { name: '영상 템플릿 삭제' }))
    const dialog = within(await screen.findByRole('dialog'))
    expect(dialog.getByText(/클립 2개/)).toBeInTheDocument()
    await user.click(dialog.getByRole('button', { name: '삭제' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/video-templates'))
    await waitFor(() =>
      expect(screen.queryByRole('link', { name: /여행/ })).not.toBeInTheDocument(),
    )
    expect(queryClient.getQueryState(projectKey)?.isInvalidated).toBe(true)
    expect(queryClient.getQueryState(otherOwnerKey)?.isInvalidated).toBe(false)
  })
  it('stays on the template and reports a failed delete', async () => {
    const user = userEvent.setup()
    const { router } = mount('/video-templates/owned', { deleteFails: true })
    await user.click(await screen.findByRole('button', { name: '더 보기' }))
    await user.click(await screen.findByRole('menuitem', { name: '영상 템플릿 삭제' }))
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', { name: '삭제' }),
    )
    // The dialog closes on failure too, so the message is not left behind the scrim.
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(router.state.location.pathname).toBe('/video-templates/owned')
    expect(await screen.findByRole('heading', { name: '여행', level: 1 })).toBeInTheDocument()
  })
})
