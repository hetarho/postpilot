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
  projectCount: 2,
}
const mount = (path: string, clips: FakeClipsOptions = {}) =>
  renderAppAt(path, { user: { id: 'alice' }, clips: { templates: [template], ...clips } })

describe('video template workflow', () => {
  it('renders empty and failed directories as text', async () => {
    const empty = mount('/video-templates', { templates: [] })
    expect(await screen.findByText('아직 저장된 영상 템플릿이 없어요')).toBeInTheDocument()
    empty.unmount()
    mount('/video-templates', { listFails: true })
    expect(await screen.findByRole('alert')).toHaveTextContent('영상 템플릿을 불러오지 못했어요.')
    expect(screen.getByRole('button', { name: '다시 시도' })).toBeInTheDocument()
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
    expect(screen.getAllByRole('img', { name: '오늘의 좋은 순간' })).toHaveLength(3)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
  it('creates one complete recipe and keeps the saved route clean', async () => {
    const user = userEvent.setup()
    const writes: ClipRecipe[] = []
    const calls: string[] = []
    const { router } = mount('/video-templates/new', { writes, calls })
    await user.type(await screen.findByLabelText('템플릿 이름'), ' 새 레시피 ')
    await user.click(screen.getByRole('button', { name: '정보 추가' }))
    await user.type(screen.getByLabelText('정보 이름 1'), '주제')
    await user.type(screen.getByLabelText('질문 안내 1'), '어떤 경험인가요?')
    await user.dblClick(screen.getByRole('button', { name: '저장' }))
    await waitFor(() =>
      expect(router.state.location.pathname).toBe('/video-templates/video-template-1'),
    )
    expect(calls.filter((v) => v === 'CreateVideoTemplate')).toHaveLength(1)
    expect(writes[0]).toMatchObject({
      name: '새 레시피',
      informationFields: [{ label: '주제', prompt: '어떤 경험인가요?' }],
      copyStyles: ['clean'],
    })
  })
  it('reorders and removes fields, refuses no style and saves the resulting recipe', async () => {
    const user = userEvent.setup()
    const writes: ClipRecipe[] = []
    mount('/video-templates/owned', { writes })
    await screen.findByLabelText('템플릿 이름')
    await user.click(screen.getAllByRole('button', { name: '아래로 이동' })[0]!)
    expect(screen.getByLabelText('정보 이름 1')).toHaveValue('음식')
    await user.click(screen.getByRole('button', { name: '정보 2 삭제' }))
    await user.click(screen.getByRole('checkbox', { name: '깔끔하게' }))
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
    await user.click(screen.getByRole('checkbox', { name: '기록처럼' }))
    await user.click(screen.getByRole('button', { name: '저장' }))
    await screen.findByText('저장했어요')
    expect(writes[0]).toMatchObject({
      informationFields: [{ label: '음식', prompt: '무엇을 먹었나요?' }],
      copyStyles: ['diary'],
    })
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
  it('confirms deletion and reports the actual detached count on the directory', async () => {
    const user = userEvent.setup()
    const { router, queryClient, transport } = mount('/video-templates/owned', { detachedCount: 3 })
    const projectKey = ['clip-projects', transport, 'alice', 'list']
    const otherOwnerKey = ['clip-projects', transport, 'bob', 'list']
    queryClient.setQueryData(projectKey, [])
    queryClient.setQueryData(otherOwnerKey, [])
    await user.click(await screen.findByRole('button', { name: '삭제' }))
    const dialog = within(await screen.findByRole('dialog'))
    expect(dialog.getByText(/클립 2개/)).toBeInTheDocument()
    await user.click(dialog.getByRole('button', { name: '삭제' }))
    expect(
      await screen.findByText('영상 템플릿을 삭제하고 클립 3개의 연결을 해제했어요.'),
    ).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/video-templates')
    expect(queryClient.getQueryState(projectKey)?.isInvalidated).toBe(true)
    expect(queryClient.getQueryState(otherOwnerKey)?.isInvalidated).toBe(false)
  })
  it('does not leave after a failed delete', async () => {
    const user = userEvent.setup()
    const { router } = mount('/video-templates/owned', { deleteFails: true })
    await user.click(await screen.findByRole('button', { name: '삭제' }))
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', { name: '삭제' }),
    )
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(router.state.location.pathname).toBe('/video-templates/owned')
    expect(screen.getByLabelText('템플릿 이름')).toHaveValue('여행')
  })
})
