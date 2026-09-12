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
    expect(screen.getAllByRole('img', { name: '오늘의 좋은 순간' })).toHaveLength(5)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
  it('creates one complete recipe and keeps the saved route clean', async () => {
    const user = userEvent.setup()
    const writes: ClipRecipe[] = []
    const calls: string[] = []
    const { router } = mount('/video-templates/new', { writes, calls })
    await user.type(await screen.findByLabelText('템플릿 이름'), ' 새 레시피 ')
    // A template names its category first: the preset fixes the chip order, the
    // default CTA and the default accent, and it seeds the reserved fields.
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
    await user.click(screen.getByRole('combobox', { name: /카테고리 프리셋/ }))
    await user.click(screen.getByRole('option', { name: '카페' }))
    await screen.findByDisplayValue('상호')
    // The four seeded fields come first, so the owner's own question is the fifth.
    await user.click(screen.getByRole('button', { name: '정보 추가' }))
    await user.type(screen.getByLabelText('정보 이름 5'), '주제')
    await user.type(screen.getByLabelText('질문 안내 5'), '어떤 경험인가요?')
    await user.dblClick(screen.getByRole('button', { name: '저장' }))
    await waitFor(() =>
      expect(router.state.location.pathname).toBe('/video-templates/video-template-1'),
    )
    expect(calls.filter((v) => v === 'CreateVideoTemplate')).toHaveLength(1)
    expect(writes[0]).toMatchObject({
      name: '새 레시피',
      preset: 'cafe',
      copyStyles: ['clean'],
    })
    // 상호 first, then the preset's own chip priority, each with its
    // code-owned prompt — the owner types no Korean label exactly.
    expect(writes[0]!.informationFields.map((f) => f.label)).toEqual([
      '상호',
      '위치',
      '메뉴',
      '영업',
      '주제',
    ])
    expect(writes[0]!.informationFields[0]!.prompt).not.toBe('')
  })
  it('reorders and removes fields, refuses no style and saves the resulting recipe', async () => {
    const user = userEvent.setup()
    const writes: ClipRecipe[] = []
    mount('/video-templates/owned', { writes })
    await screen.findByLabelText('템플릿 이름')
    await user.click(screen.getAllByRole('button', { name: '아래로 이동' })[0]!)
    expect(screen.getByLabelText('정보 이름 1')).toHaveValue('음식')
    await user.click(screen.getByRole('button', { name: '정보 2 삭제' }))
    // 깔끔하게 cannot be turned off at all: every CDS fallback lands on it.
    expect(screen.getByRole('checkbox', { name: '깔끔하게' })).toBeDisabled()
    expect(screen.getByRole('checkbox', { name: '깔끔하게' })).toBeChecked()
    await user.click(screen.getByRole('checkbox', { name: '메모' }))
    await user.click(screen.getByRole('checkbox', { name: '가벼운 텍스트' }))
    await user.click(screen.getByRole('combobox', { name: /자막 흐름/ }))
    await user.click(screen.getByRole('option', { name: '빠른 구절형' }))
    await user.click(screen.getByRole('button', { name: '저장' }))
    await screen.findByText('저장했어요')
    expect(writes[0]).toMatchObject({
      informationFields: [{ label: '음식', prompt: '무엇을 먹었나요?' }],
      copyStyles: ['clean', 'memo', 'simple'],
      captionPace: 'rapid',
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
  it('warns before seeding a template that clips already use, and keeps existing labels', async () => {
    const user = userEvent.setup()
    const writes: ClipRecipe[] = []
    mount('/video-templates/owned', { writes })
    await screen.findByLabelText('템플릿 이름')
    // The fixture template is used by two clips, so adding the preset's
    // reserved fields changes what their step ① asks for.
    await user.click(screen.getByRole('combobox', { name: /카테고리 프리셋/ }))
    await user.click(screen.getByRole('option', { name: '카페' }))
    expect(await screen.findByText(/정보 항목을 추가할까요/)).toBeInTheDocument()
    expect(screen.queryByDisplayValue('상호')).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '추가' }))
    await screen.findByDisplayValue('상호')
    // The owner's own questions are kept, and only missing reserved labels are
    // added — 장소 and 음식 are still there, in their own order.
    expect(screen.getByLabelText('정보 이름 1')).toHaveValue('장소')
    expect(screen.getByLabelText('정보 이름 2')).toHaveValue('음식')
    await user.click(screen.getByRole('button', { name: '저장' }))
    await screen.findByText('저장했어요')
    expect(writes[0]!.informationFields.map((f) => f.label)).toEqual([
      '장소',
      '음식',
      '상호',
      '위치',
      '메뉴',
      '영업',
    ])
    expect(writes[0]!.preset).toBe('cafe')
  })
  it('seeds without a dialog when no clip uses the template yet', async () => {
    const user = userEvent.setup()
    mount('/video-templates/owned', { templates: [{ ...template, projectCount: 0 }] })
    await screen.findByLabelText('템플릿 이름')
    await user.click(screen.getByRole('combobox', { name: /카테고리 프리셋/ }))
    await user.click(screen.getByRole('option', { name: '카페' }))
    await screen.findByDisplayValue('상호')
    expect(screen.queryByText(/정보 항목을 추가할까요/)).not.toBeInTheDocument()
  })
})
