import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import type { FakeClipsOptions } from '@/test/clips'
import type { ClipRecipe } from '@/entities/clip-template'

const template = {
  id: 'owned',
  name: '여행',
  cutGuidance: '풍경 위주',
  informationFields: [
    { label: '장소', prompt: '어디인가요?' },
    { label: '음식', prompt: '무엇을 먹었나요?' },
  ],
  copyStyles: ['clean'] as ClipRecipe['copyStyles'],
  accent: '' as const,
  preset: 'restaurant' as const,
  projectCount: 2,
}
const mount = (path: string, clips: FakeClipsOptions = {}) =>
  renderAppAt(path, { user: { id: 'alice' }, clips: { templates: [template], ...clips } })

describe('video template workflow', () => {
  it('renders empty and failed directories the way the post templates do', async () => {
    const empty = mount('/video-templates', { templates: [] })
    // Two parts, like `/templates`: what is missing, and what a video template is FOR.
    expect(
      await screen.findByRole('heading', { name: '아직 저장된 영상 템플릿이 없어요' }),
    ).toBeInTheDocument()
    expect(screen.getByText(/클립을 만들 때 받을 정보와 컷 구성/)).toBeInTheDocument()
    empty.unmount()
    mount('/video-templates', { listFails: true })
    expect(await screen.findByRole('alert')).toHaveTextContent('영상 템플릿을 불러오지 못했어요.')
    expect(screen.getByRole('button', { name: '다시 시도' })).toBeInTheDocument()
  })
  it('badges how many clips each template is used by, and docks one CTA', async () => {
    mount('/video-templates')
    const region = within(await screen.findByRole('region', { name: '저장된 영상 템플릿' }))
    expect(region.getByText('클립 2개')).toBeInTheDocument()
    expect(screen.getAllByRole('link', { name: '새 영상 템플릿' })).toHaveLength(1)
  })
  it("heads the detail with the template's own name, and the create screen with its own", async () => {
    const editing = mount('/video-templates/owned')
    expect(await screen.findByRole('heading', { name: '여행', level: 1 })).toBeInTheDocument()
    editing.unmount()
    mount('/video-templates/new')
    expect(await screen.findByRole('heading', { name: '새 영상 템플릿' })).toBeInTheDocument()
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
    expect(await screen.findByLabelText('템플릿 이름')).toHaveValue('여행')
    expect(router.state.location.pathname).toBe('/video-templates/owned')
    expect(
      screen.getByRole('img', { name: '예시 영상의 안전 영역과 현재 문구 구성' }),
    ).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('keeps edits after a localized server refusal', async () => {
    const user = userEvent.setup()
    mount('/video-templates/owned', { saveFails: true })
    const name = await screen.findByLabelText('템플릿 이름')
    await user.clear(name)
    await user.type(name, '실패해도 보존')
    await user.click(screen.getByRole('button', { name: '저장' }))
    expect(await screen.findByText('같은 이름의 영상 템플릿이 이미 있어요.')).toBeInTheDocument()
    expect(name).toHaveValue('실패해도 보존')
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
  // The delete rides the directory ROW now, where the post-template directory has it (CLIP-42),
  // and the detach is warned about BEFORE the delete rather than reported on the screen it lands
  // on — a warning the owner reads after the fact is not a warning.
  it('warns how many clips a delete detaches, and removes the row', async () => {
    const user = userEvent.setup()
    const { router, queryClient, transport } = mount('/video-templates')
    const projectKey = ['clip-projects', transport, 'alice', 'list']
    const otherOwnerKey = ['clip-projects', transport, 'bob', 'list']
    queryClient.setQueryData(projectKey, [])
    queryClient.setQueryData(otherOwnerKey, [])
    await user.click(await screen.findByRole('button', { name: '여행 삭제' }))
    const dialog = within(await screen.findByRole('dialog'))
    expect(dialog.getByText(/클립 2개/)).toBeInTheDocument()
    await user.click(dialog.getByRole('button', { name: '삭제' }))
    await waitFor(() =>
      expect(screen.queryByRole('link', { name: /여행/ })).not.toBeInTheDocument(),
    )
    // The delete acts without navigating: a row is one target, not a row with a button inside it.
    expect(router.state.location.pathname).toBe('/video-templates')
    expect(queryClient.getQueryState(projectKey)?.isInvalidated).toBe(true)
    expect(queryClient.getQueryState(otherOwnerKey)?.isInvalidated).toBe(false)
  })
  it('keeps the row and reports a failed delete beside its own trigger', async () => {
    const user = userEvent.setup()
    const { router } = mount('/video-templates', { deleteFails: true })
    await user.click(await screen.findByRole('button', { name: '여행 삭제' }))
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', { name: '삭제' }),
    )
    // The sheet closes on failure too, so the message is not left behind the scrim.
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(router.state.location.pathname).toBe('/video-templates')
    expect(await screen.findByRole('link', { name: /여행/ })).toBeInTheDocument()
  })
})
