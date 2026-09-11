import { expect, it, describe } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import type { FakeClipProject, FakeClipsOptions, FakeClipTemplate } from '@/test/clips'
import type { FakeGenerationJobRow } from '@/test/jobs'

const template: FakeClipTemplate = {
  id: 'template',
  name: '여행',
  informationFields: [],
  cutGuidance: '',
  copyStyles: ['clean'],
  accent: '',
  preset: 'restaurant',
}
const result = {
  contentType: 'video/mp4',
  bytes: 5,
  durationMs: 15000,
  createdAt: '2026-09-10T00:00:00Z',
}
const base = {
  videoTemplateId: 'template',
  ratio: 'vertical' as const,
  targetDurationMs: 15000,
  disclosure: 'ad' as const,
  cta: '' as const,
  answers: [],
}
const running: FakeGenerationJobRow = {
  id: 'running-job',
  kind: 'generate_clip',
  clipProjectId: 'running',
  status: 'running',
  stage: 'analyze',
}
const failed: FakeGenerationJobRow = {
  id: 'failed-job',
  kind: 'generate_clip',
  clipProjectId: 'failed',
  status: 'failed',
  stage: 'prepare',
}
const projects: FakeClipProject[] = [
  { ...base, id: 'draft', title: '제주 여행' },
  {
    ...base,
    id: 'refining',
    title: '서울 카페',
    editPlanRevision: 2,
    renderedPlanRevision: 1,
    result,
  },
  {
    ...base,
    id: 'finished',
    title: 'JEJU 다시',
    editPlanRevision: 1,
    renderedPlanRevision: 1,
    result,
  },
  { ...base, id: 'running', title: '부산 바다', latestJob: running },
  {
    ...base,
    id: 'failed',
    title: '강릉 바다',
    latestJob: failed,
    editPlanRevision: 1,
    renderedPlanRevision: 1,
    result,
  },
]
const mount = (path = '/clips', clips: FakeClipsOptions = {}) =>
  renderAppAt(path, {
    user: { id: 'alice' },
    clips: { templates: [{ ...template }], projects, ...clips },
  })
const row = async (name: RegExp) =>
  within(await screen.findByRole('list', { name: '저장된 클립' })).findByRole('link', { name })

describe('clip directory', () => {
  it('badges every row from the project and its latest job', async () => {
    mount()
    // A job in flight and a failed attempt outrank the project's own state (CLIP-41).
    expect(await row(/부산 바다/)).toHaveTextContent('생성 중')
    expect(await row(/강릉 바다/)).toHaveTextContent('실패')
    expect(await row(/제주 여행/)).toHaveTextContent('초안')
    expect(await row(/서울 카페/)).toHaveTextContent('다듬는 중')
    expect(await row(/JEJU 다시/)).toHaveTextContent('완성')
  })

  it('carries the template, the ratio and a relative time on the row', async () => {
    mount()
    const link = await row(/제주 여행/)
    await waitFor(() => expect(link).toHaveTextContent('여행'))
    expect(link).toHaveTextContent('세로 9:16')
    expect(within(link).getByText(/전|어제|방금/)).toBeInTheDocument()
  })

  it('narrows by title and by state, and carries both in the URL', async () => {
    const user = userEvent.setup()
    const { router } = mount()
    await user.type(await screen.findByLabelText('클립 검색'), '바다')
    await waitFor(() => expect(router.state.location.search).toMatchObject({ q: '바다' }))
    expect(screen.queryByRole('link', { name: /제주 여행/ })).not.toBeInTheDocument()
    expect(await row(/부산 바다/)).toBeInTheDocument()

    await user.click(screen.getByRole('tab', { name: '완성' }))
    await waitFor(() => expect(router.state.location.search).toMatchObject({ status: 'finished' }))
    // 강릉 바다 is finished AND failed: the filter reads the project's own state, not the badge.
    expect(await row(/강릉 바다/)).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /부산 바다/ })).not.toBeInTheDocument()
  })

  it('drops an emptied query from the address rather than carrying it', async () => {
    const user = userEvent.setup()
    const { router } = mount('/clips?q=바다')
    const search = await screen.findByLabelText('클립 검색')
    expect(search).toHaveValue('바다')
    await user.clear(search)
    await waitFor(() => expect(router.state.location.search).toEqual({}))
  })

  it('says what is narrowing when nothing matches, and offers the way back', async () => {
    const user = userEvent.setup()
    // 강릉 바다 exists, but it is finished — so this pair matches nothing.
    const { router } = mount('/clips?q=강릉&status=draft')
    expect(await screen.findByText(/'강릉'와 맞는 초안 클립이 없어요/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '전체 보기' }))
    await waitFor(() => expect(router.state.location.search).toEqual({}))
    expect(await row(/제주 여행/)).toBeInTheDocument()
  })

  it('ignores a status the list cannot honour', async () => {
    mount('/clips?status=nonsense')
    // Dropped at the route, so the directory renders rather than refusing (CLIP-41).
    expect(await row(/제주 여행/)).toBeInTheDocument()
  })

  it('reports a failed directory in a notice with a retry, and an empty one in a live region', async () => {
    const failing = mount('/clips', { projectListFails: true })
    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '다시 시도' })).toBeInTheDocument()
    failing.unmount()

    mount('/clips', { projects: [] })
    expect(await screen.findByText('아직 저장된 클립이 없어요')).toBeInTheDocument()
  })

  it('docks exactly one 새 클립', async () => {
    mount()
    await screen.findByRole('list', { name: '저장된 클립' })
    expect(screen.getAllByRole('link', { name: '새 클립' })).toHaveLength(1)
  })
})
