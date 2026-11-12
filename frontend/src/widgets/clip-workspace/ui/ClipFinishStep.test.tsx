import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import { afterEach, expect, it, vi } from 'vitest'
import { ClipProjectSchema } from '@/shared/api'
import { toClipProject } from '@/entities/clip-project'
import type { BrowserRenderState } from '@/features/render-clip-browser'
import { createTestQueryClient, withProviders } from '@/test/session'
import { renderAppAt } from '@/test/app'
import { clipTimelineFixture } from '@/test/clip-editing'
import type { FakeClipProject } from '@/test/clips'
import type { FakeDraftSave } from '@/test/posts'
import { ClipFinishStep, type ClipFinishRender } from './ClipFinishStep'

afterEach(cleanup)

const result = {
  id: 'render-1',
  contentType: 'video/mp4',
  createdAt: '2026-11-12T00:00:00Z',
  viewUrl: 'https://example.test/view',
  downloadUrl: 'https://example.test/download',
}

function clip(overrides: Record<string, unknown> = {}) {
  return toClipProject(
    create(ClipProjectSchema, {
      id: 'clip',
      ratio: 'vertical',
      channel: 'naver_clip',
      editPlanRevision: 2,
      confirmedPlanRevision: 2,
      canFinalize: true,
      ...overrides,
    }),
  )
}

function renderHandle(overrides: Partial<ClipFinishRender> = {}, phase = 'idle') {
  return {
    browser: {
      state: {
        phase,
        progress: { stage: 'encoding', percent: 40 },
      } as BrowserRenderState,
      cancel: vi.fn(),
      retry: vi.fn(),
    },
    capability: { available: true },
    ready: true,
    pending: false,
    serverEntitled: true,
    serverPlan: 'max',
    serverWindow: {
      coverageId: 'max',
      startsAt: '',
      endsAt: '',
      allowance: 60,
      used: 0,
      reserved: 0,
      remaining: 60,
    },
    start: vi.fn(),
    ...overrides,
  } as ClipFinishRender
}

function finalizationFor(project: ReturnType<typeof clip>) {
  return {
    prepare: vi.fn(async () => project),
    confirm: vi.fn<(url?: string) => Promise<void>>(async () => {}),
    checkAgain: vi.fn(async () => {}),
    savePublishedUrl: vi.fn<(url: string) => Promise<boolean>>(async () => true),
    pending: false,
    uncertain: false,
    failure: undefined,
    busy: false,
  }
}

function show({
  project = clip(),
  handle = renderHandle(),
  startRender = false,
  slots,
}: {
  project?: ReturnType<typeof clip>
  handle?: ClipFinishRender
  startRender?: boolean
  slots?: Parameters<typeof ClipFinishStep>[0]['slots']
} = {}) {
  const handled = vi.fn()
  const goRefine = vi.fn()
  const finalization = finalizationFor(project)
  render(
    <ClipFinishStep
      ownerId="owner"
      project={project}
      render={handle}
      finalization={finalization}
      disabled={false}
      startRender={startRender}
      onStartRenderHandled={handled}
      onGoRefine={goRefine}
      slots={slots}
    />,
    {
      wrapper: withProviders(
        createRouterTransport(() => {}),
        createTestQueryClient(),
      ),
    },
  )
  return { handled, goRefine, finalization, handle }
}

it('opens on 완성됐어요, then the delivered render, the T767 slots and the URL at its foot', () => {
  show({
    project: clip({ renderedPlanRevision: 2, result }),
    slots: {
      checklist: <p>checklist slot</p>,
      uploadText: <p>upload text slot</p>,
      more: <p>more slot</p>,
    },
  })
  const order = [
    screen.getByRole('heading', { name: '완성됐어요' }),
    screen.getByLabelText(/미리보기|preview/i, { selector: 'video' }),
    screen.getByRole('link', { name: '렌더 2 다운로드' }),
    screen.getByText('checklist slot'),
    screen.getByText('upload text slot'),
    screen.getByText('more slot'),
    screen.getByRole('heading', { name: '게시 URL' }),
    screen.getByRole('button', { name: '완료로 표시' }),
  ]
  for (let i = 1; i < order.length; i++)
    expect(order[i - 1].compareDocumentPosition(order[i]) & Node.DOCUMENT_POSITION_FOLLOWING).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING,
    )
  expect(screen.getByText('수정본 2의 영상')).toBeInTheDocument()
  // A matching render offers no other render, and nothing runs, so no step bar docks.
  expect(screen.queryByRole('button', { name: '렌더하기' })).not.toBeInTheDocument()
  expect(screen.queryByLabelText('렌더 작업')).not.toBeInTheDocument()
})

it('starts the browser render of the confirmed plan at once after 확정하기', () => {
  const { handle, handled } = show({ startRender: true })
  expect(handle.start).toHaveBeenCalledExactlyOnceWith('browser')
  expect(handled).toHaveBeenCalled()
})

it('skips the automatic render when a durable render already matches', () => {
  const { handle, handled } = show({
    project: clip({ renderedPlanRevision: 2, result }),
    startRender: true,
  })
  expect(handle.start).not.toHaveBeenCalled()
  expect(handled).toHaveBeenCalled()
})

it('waits for an unknown device and leaves an unsupported one to the offer', () => {
  const unknown = show({ startRender: true, handle: renderHandle({ capability: undefined }) })
  expect(unknown.handle.start).not.toHaveBeenCalled()
  expect(unknown.handled).not.toHaveBeenCalled()
  cleanup()
  const refused = show({
    startRender: true,
    handle: renderHandle({ capability: { available: false, reason: 'capability' } }),
  })
  expect(refused.handle.start).not.toHaveBeenCalled()
  expect(refused.handled).toHaveBeenCalled()
  expect(screen.getByRole('button', { name: '렌더하기' })).toBeInTheDocument()
})

it('offers 렌더하기 with the server as an explicit choice', async () => {
  const { handle } = show()
  await userEvent.click(screen.getByRole('button', { name: '렌더하기' }))
  const choice = within(await screen.findByRole('dialog', { name: '렌더하기' }))
  await userEvent.click(choice.getByRole('button', { name: '서버에서 렌더' }))
  expect(handle.start).toHaveBeenCalledExactlyOnceWith('server')
})

it('docks the progress and cancellation while a browser render runs', () => {
  const { handle } = show({ handle: renderHandle({}, 'running') })
  const bar = screen.getByLabelText('렌더 작업')
  expect(within(bar).getByRole('progressbar')).toBeInTheDocument()
  fireEvent.click(within(bar).getByRole('button', { name: '브라우저 렌더 취소' }))
  expect(handle.browser.cancel).toHaveBeenCalled()
  expect(screen.queryByRole('button', { name: '렌더하기' })).not.toBeInTheDocument()
})

it('names the action 다시 렌더 in the step bar after a failure, and in flow after a cancellation', () => {
  show({ handle: renderHandle({}, 'failed') })
  const bar = screen.getByLabelText('렌더 작업')
  expect(within(bar).getByRole('button', { name: '다시 렌더' })).toBeInTheDocument()
  cleanup()
  show({ handle: renderHandle({}, 'cancelled') })
  expect(screen.queryByLabelText('렌더 작업')).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: '다시 렌더' })).toBeInTheDocument()
  cleanup()
  show({ handle: renderHandle({ server: { outcome: 'failed' } }) })
  expect(
    within(screen.getByLabelText('렌더 작업')).getByRole('button', {
      name: '다시 렌더',
    }),
  ).toBeInTheDocument()
})

it('waits for 확정하기 on an unconfirmed plan and keeps the earlier revision playable', () => {
  const { goRefine, handle } = show({
    project: clip({
      editPlanRevision: 3,
      renderedPlanRevision: 1,
      confirmedPlanRevision: 2,
      result,
    }),
    startRender: true,
  })
  expect(screen.getByRole('heading', { name: '확정하기를 기다리고 있어요' })).toBeInTheDocument()
  expect(screen.getByText('이전 수정본 1의 영상이에요')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: '렌더 1 다운로드' })).toHaveAttribute(
    'href',
    result.downloadUrl,
  )
  expect(screen.queryByRole('button', { name: '렌더하기' })).not.toBeInTheDocument()
  expect(handle.start).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('button', { name: '영상 다듬기로 가기' }))
  expect(goRefine).toHaveBeenCalled()
})

it('finalizes with the posted URL or 완료로 표시 through one dialog, and cancelling starts nothing', async () => {
  const { finalization } = show({ project: clip({ renderedPlanRevision: 2, result }) })
  fireEvent.click(screen.getByRole('button', { name: '완료로 표시' }))
  let dialog = within(await screen.findByRole('dialog'))
  fireEvent.click(dialog.getByRole('button', { name: /취소/ }))
  expect(finalization.confirm).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('button', { name: '완료로 표시' }))
  dialog = within(await screen.findByRole('dialog'))
  fireEvent.click(dialog.getByRole('button', { name: '완료로 표시' }))
  expect(finalization.confirm).toHaveBeenLastCalledWith(undefined)
  fireEvent.change(screen.getByLabelText('네이버 클립 주소'), {
    target: { value: 'https://clip.naver.com/v/9' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'URL 저장' }))
  dialog = within(await screen.findByRole('dialog'))
  fireEvent.click(dialog.getByRole('button', { name: '저장하고 완료' }))
  expect(finalization.confirm).toHaveBeenLastCalledWith('https://clip.naver.com/v/9')
})

it('keeps playback, download and a replaceable URL on a finalized clip', async () => {
  const { finalization } = show({
    project: clip({
      renderedPlanRevision: 2,
      result,
      finalizedAt: '2026-11-12T01:00:00Z',
      finalizedPlanRevision: 2,
      finalizedResultId: result.id,
    }),
  })
  expect(screen.getByRole('heading', { name: '완성됐어요' })).toBeInTheDocument()
  expect(screen.getByRole('link', { name: '렌더 2 다운로드' })).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '완료로 표시' })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '렌더하기' })).not.toBeInTheDocument()
  fireEvent.change(screen.getByLabelText('네이버 클립 주소'), {
    target: { value: 'https://naver.me/x' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'URL 바꾸기' }))
  expect(finalization.savePublishedUrl).toHaveBeenCalledWith('https://naver.me/x')
})

/** A confirmed sponsored clip with a storyline, for ③'s 더 해 보기 (CLIP-233). */
const confirmedClip = (overrides: Partial<FakeClipProject> = {}): FakeClipProject => ({
  id: 'clip',
  title: '성수 카페 영상',
  instruction: '라떼 위주로',
  videoTemplateId: '',
  ratio: 'vertical',
  targetDurationMs: 19800,
  disclosure: '',
  campaign: true,
  advertiserGuide: { text: '주차 안내', keywords: ['크림라떼'], hashtags: ['성수카페'] },
  editPlanRevision: 1,
  confirmed: { planRevision: 1, at: '2026-11-12T00:00:00Z' },
  editing: clipTimelineFixture(),
  storyline: {
    paragraphs: [{ text: '입구를 보여 줘요. 간판이 커요.', observationIds: [] }],
    editedByHand: false,
    addedSourceIds: [],
    takenOutObservationIds: [],
  },
  ...overrides,
})

it('holds every 더 해 보기 action until the plan is confirmed', async () => {
  renderAppAt('/clips/clip', {
    user: { id: 'alice' },
    clips: { projects: [confirmedClip({ editPlanRevision: 2 })] },
  })
  await userEvent.click(await screen.findByRole('tab', { name: '영상 완성' }))
  const card = await screen.findByRole('region', { name: '더 해 보기' })
  expect(within(card).getByText('확정하기를 하면 쓸 수 있어요.')).toBeVisible()
  expect(
    within(card).getByRole('button', { name: '이 영상 형식으로 템플릿 만들기' }),
  ).toBeDisabled()
  expect(within(card).getByRole('button', { name: '이 체험으로 글 쓰기' })).toBeDisabled()
  expect(within(card).getByRole('button', { name: '인스타 클립 업로드 문구 보기' })).toBeDisabled()
  // Neither the checklist nor the 업로드 문구 stands before confirmation.
  expect(screen.queryByRole('heading', { name: '제출 전 체크리스트' })).not.toBeInTheDocument()
  expect(screen.queryByRole('region', { name: '업로드 문구' })).not.toBeInTheDocument()
})

it('links the wizard with the clip, saves a checklist tick alone and starts nothing', async () => {
  const calls: string[] = [],
    projectRequests: Array<{ id: string; linkedPostSlug?: string; fields: string[] }> = []
  renderAppAt('/clips/clip', {
    user: { id: 'alice' },
    clips: { projects: [confirmedClip()], calls, projectRequests },
  })
  const card = await screen.findByRole('region', { name: '더 해 보기' })
  const link = within(card).getByRole('link', { name: '이 영상 형식으로 템플릿 만들기' })
  expect(link.getAttribute('href')).toMatch(/^\/video-templates\/new\?/)
  const search = new URLSearchParams(link.getAttribute('href')!.split('?')[1])
  expect(search.get('channel')).toBe('naver_clip')
  expect(search.get('from')).toBe('clip')
  // The checklist reads the confirmed plan, and a tick is saved as its own option save.
  expect(screen.getByRole('heading', { name: '제출 전 체크리스트' })).toBeVisible()
  await userEvent.click(screen.getByRole('checkbox', { name: '주차 안내' }))
  await waitFor(() => expect(projectRequests.at(-1)?.fields).toEqual(['id', 'checklistTicks']))
  expect(calls.some((call) => /^(Start|Quote|Confirm|Finalize)/u.test(call))).toBe(false)
})

it('writes a post from the same visit with no video and opens it', async () => {
  const draftSaves: FakeDraftSave[] = [],
    draftMaterials: Array<{ slug: string; title: string; memo: string }> = [],
    calls: string[] = []
  const { router } = renderAppAt('/clips/clip', {
    user: { id: 'alice' },
    clips: { projects: [confirmedClip()], calls },
    posts: { draftSaves, draftMaterials },
  })
  const card = await screen.findByRole('region', { name: '더 해 보기' })
  await userEvent.click(within(card).getByRole('button', { name: '이 체험으로 글 쓰기' }))
  await waitFor(() => expect(router.state.location.pathname).toMatch(/^\/posts\/[^/]+$/))
  expect(router.state.location.pathname).not.toBe('/posts/new')
  expect(draftSaves).toHaveLength(1)
  expect(draftMaterials[0]).toEqual({ slug: '', title: '성수 카페 영상', memo: '라떼 위주로' })
  expect(draftSaves[0]).toMatchObject({
    slug: '',
    targetLanguage: 'ko',
    campaign: true,
    advertiserGuide: { text: '주차 안내', keywords: ['크림라떼'], hashtags: ['성수카페'] },
  })
  // The clip itself is not written and no video moves.
  expect(calls.filter((call) => /^(Update|Create|Start|Confirm|Finalize)/u.test(call))).toEqual([])
})
