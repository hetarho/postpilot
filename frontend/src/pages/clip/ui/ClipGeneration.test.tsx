import { afterEach, expect, it, vi } from 'vitest'
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createConnectQueryKey } from '@connectrpc/connect-query'
import { initializeI18n } from '@/app/providers/i18n'
import { endSession } from '@/app/model/end-session'
import { readSourceManifest } from '@/features/upload-clip-sources'
import { putBlobWithProgress } from '@/shared/lib/upload'
import { GenerationService, Stage, ProtoPlan } from '@/shared/api'
import { clipProjectsKey, type ClipAccounting } from '@/entities/clip-project'
import { renderAppAt } from '@/test/app'
import type { FakeClipProject, FakeClipsOptions } from '@/test/clips'
import type { FakeGenerationJobRow, FakeJobsOptions } from '@/test/jobs'
import type { FakeProvidersOptions } from '@/test/providers'
import type { FakePlansOptions } from '@/test/plans'

vi.mock('@/features/upload-clip-sources/model/manifest', async (original) => ({
  ...(await original<object>()),
  readSourceManifest: vi.fn(),
}))
vi.mock('@/shared/lib/upload', async (original) => ({
  ...(await original<object>()),
  putBlobWithProgress: vi.fn(),
}))
afterEach(() => {
  vi.restoreAllMocks()
  initializeI18n('ko')
})

const project: FakeClipProject = {
  id: 'clip',
  title: '제주',
  videoTemplateId: 'template',
  ratio: 'vertical',
  targetDurationMs: 15000,
  disclosure: 'ad',
  cta: '',
  answers: [],
}
const result = {
  contentType: 'video/mp4',
  bytes: 5,
  durationMs: 15000,
  createdAt: '2026-09-10T00:00:00Z',
  viewUrl: 'https://private.test/view',
  downloadUrl: 'https://private.test/download',
}
const models: FakeProvidersOptions = {
  models: [
    {
      providerId: 'p',
      modelId: 'o',
      label: 'Video observer',
      vision: true,
      videoInput: true,
      inlineStaticVideo: true,
      stages: [Stage.OBSERVE],
    },
    { providerId: 'p', modelId: 'w', label: 'Writer', stages: [Stage.WRITE] },
  ],
  selections: [
    { stage: Stage.OBSERVE, providerId: 'p', modelId: 'o' },
    { stage: Stage.WRITE, providerId: 'p', modelId: 'w' },
  ],
}
function mount(
  clips: FakeClipsOptions = {},
  jobs: FakeJobsOptions = {},
  providers: FakeProvidersOptions = models,
  plans?: FakePlansOptions,
) {
  return renderAppAt('/clips/clip', {
    user: { id: 'alice' },
    providers,
    plans,
    jobs,
    clips: {
      templates: [
        {
          id: 'template',
          name: '여행',
          informationFields: [],
          cutGuidance: '',
          copyStyles: ['clean'],
          accent: '',
          preset: 'restaurant',
        },
      ],
      projects: [project],
      // T111's live answer: the fixture observer is eligible unless a test says otherwise.
      eligibility: [{ providerId: 'p', modelId: 'o', status: 'eligible' }],
      ...clips,
    },
  })
}
/** The workspace is three panels behind one tab row now (CLIP-36), so a test that asserts across
 *  steps has to say which one it is looking at. */
async function goToStep(name: '클립 생성' | '클립 다듬기' | '클립 완성') {
  await userEvent.setup().click(await screen.findByRole('tab', { name }))
}
async function selectSource() {
  const file = new File(['clip'], 'clip.mp4', { type: 'video/mp4' })
  vi.mocked(readSourceManifest).mockResolvedValue([
    {
      filename: file.name,
      contentType: file.type,
      bytes: file.size,
      width: 1920,
      height: 1080,
      durationMs: 15000,
      fingerprint: 'a'.repeat(64),
    },
  ])
  vi.mocked(putBlobWithProgress).mockResolvedValue()
  vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:clip-source')
  const revoke = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
  const input = await screen.findByLabelText(/^원본 영상 (다시 )?선택$/)
  await waitFor(() => expect(input).toBeEnabled())
  const user = userEvent.setup()
  await user.upload(input, file)
  await screen.findByText('업로드 확인 완료')
  return { user, revoke }
}
it('approves once, retains local previews after terminal and refetches the result', async () => {
  const calls: string[] = []
  const starts: unknown[] = []
  const job: FakeGenerationJobRow = {
    id: 'clip-job',
    kind: 'generate_clip',
    clipProjectId: 'clip',
    status: 'running',
    stage: 'analyze',
    progressDone: 1,
    progressTotal: 3,
  }
  let finished = false
  const view = mount(
    {
      calls,
      generationStarts: starts,
      readProject: (p) => (finished ? { ...p, result, latestJob: job } : p),
    },
    {
      jobs: [job],
      onRead: (j) => {
        finished = j.status === 'done'
      },
    },
  )
  const { user, revoke } = await selectSource()
  const generate = await screen.findByRole('button', { name: '최대 20 크레딧 · 승인하고 생성' })
  await waitFor(() => expect(generate).toBeEnabled())
  await user.dblClick(generate)
  await screen.findByText('영상 분석')
  expect(starts).toHaveLength(1)
  expect(starts[0]).toMatchObject({
    projectId: 'clip',
    observeModel: { providerId: 'p', modelId: 'o' },
    writeModel: { providerId: 'p', modelId: 'w' },
    quoteId: 'quote-1',
    approvedMaxCredits: 20,
  })
  expect(revoke).not.toHaveBeenCalled()
  expect(screen.getByText('clip.mp4')).toBeInTheDocument()
  expect(screen.getByLabelText('clip.mp4')).toHaveAttribute('src', 'blob:clip-source')
  expect(screen.getByLabelText('클립 제목')).toBeDisabled()
  expect(screen.getByRole('button', { name: '삭제' })).toBeDisabled()
  expect(screen.getByRole('combobox', { name: /관찰/ })).toBeDisabled()
  expect(screen.getAllByRole('progressbar')).toHaveLength(1)
  expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '1')
  expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuemax', '3')
  job.status = 'done'
  job.stage = 'cleanup'
  await act(async () => {
    await view.queryClient.refetchQueries({
      queryKey: createConnectQueryKey({
        schema: GenerationService.method.getGeneration,
        input: { id: 'clip-job' },
        transport: view.transport,
        cardinality: 'finite',
      }),
    })
  })
  // A finished render describes the project as 완성, so the bar follows to ③ where the result is.
  const video = await screen.findByLabelText('클립 미리보기')
  expect(revoke).not.toHaveBeenCalled()
  expect(screen.queryByLabelText('clip.mp4')).not.toBeInTheDocument()
  expect(video).toHaveAttribute('src', result.viewUrl)
  expect(video).toHaveAttribute('controls')
  expect(video).toHaveAttribute('preload', 'metadata')
  expect(screen.getByRole('link', { name: '영상 다운로드' })).toHaveAttribute(
    'href',
    result.downloadUrl,
  )
  expect(calls.filter((c) => c === 'GetClipProject').length).toBeGreaterThan(1)
  expect(calls).not.toContain('DiscardClipSourceBatch')
  // The consumed selection's summary stays with the picker that made it, on ①.
  await goToStep('클립 생성')
  expect(screen.getByText('clip.mp4 · 처리 완료')).toBeInTheDocument()
  // A new selection after completion must not be mistaken for the consumed batch.
  await selectSource()
  expect(
    await screen.findByRole('button', { name: '최대 20 크레딧 · 승인하고 생성' }),
  ).toBeEnabled()
})
it('keeps an older result visible on a durable credit refusal and requires a new batch', async () => {
  const job: FakeGenerationJobRow = {
    id: 'clip-job',
    kind: 'generate_clip',
    status: 'failed',
    stage: 'prepare',
    failureReason: 'INSUFFICIENT_CREDITS',
    failureParams: { required: '79', balance: '12', renews_at: '2026-09-30T15:00:00Z' },
  }
  mount({ projects: [{ ...project, result, latestJob: job }] }, { jobs: [job] })
  // A failed attempt opens on the step that owns its retry (CLIP-26).
  expect(await screen.findByText(/크레딧이 79 필요한데 12만 남았어요/)).toBeInTheDocument()
  expect(screen.getByText('원본 확인 단계에서 실패했어요')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: '크레딧·요금제 확인' })).toHaveAttribute('href', '/plans')
  expect(screen.getByRole('button', { name: '다시 생성' })).toBeDisabled()
  // The previous successful result is untouched, one tab away (CLIP-26).
  await goToStep('클립 완성')
  expect(await screen.findByLabelText('클립 미리보기')).toHaveAttribute('src', result.viewUrl)
  await goToStep('클립 생성')
  await selectSource()
  expect(
    await screen.findByRole('button', { name: '최대 20 크레딧 · 승인하고 생성' }),
  ).toBeEnabled()
})
it('reloads saved results without originals and refreshes an expired preview only once automatically', async () => {
  let reads = 0
  const view = mount({
    projects: [{ ...project, result }],
    readProject: (p) => ({
      ...p,
      result: { ...result, viewUrl: `https://private.test/view-${++reads}` },
    }),
  })
  const video = await screen.findByLabelText('클립 미리보기')
  expect(screen.queryByText('clip.mp4')).not.toBeInTheDocument()
  expect(screen.getByRole('link', { name: '영상 다운로드' })).toBeInTheDocument()
  const initial = reads
  fireEvent.error(video)
  await waitFor(() => expect(reads).toBe(initial + 1))
  await waitFor(() =>
    expect(screen.queryByText('미리보기 링크를 새로 불러오는 중이에요')).not.toBeInTheDocument(),
  )
  fireEvent.error(screen.getByLabelText('클립 미리보기'))
  await screen.findByText('영상을 불러오지 못했어요. 다시 불러오거나 다운로드해 주세요.')
  expect(reads).toBe(initial + 1)
  expect(view.queryClient.getMutationCache().getAll()).toHaveLength(0)
})
it('blocks an observer the server refuses, says why, and never substitutes another model', async () => {
  const starts: unknown[] = []
  const calls: string[] = []
  mount({
    generationStarts: starts,
    calls,
    eligibility: [{ providerId: 'p', modelId: 'o', status: 'video_input_absent' }],
  })
  await selectSource()
  expect(screen.getByRole('button', { name: '생성' })).toBeDisabled()
  expect(starts).toHaveLength(0)
  expect(calls).not.toContain('QuoteClipGeneration')
  // The saved choice stays selected and the reason sits beside the field and in the
  // action's readiness line; nothing was saved over it.
  const observe = screen.getByRole('combobox', { name: /관찰 모델/ })
  expect(observe).toHaveTextContent('Video observer')
  expect(screen.getAllByText('영상 입력을 받지 않는 모델이에요').length).toBeGreaterThanOrEqual(1)
  expect(observe).toHaveAccessibleDescription(/영상 입력을 받지 않는 모델이에요/)
  expect(calls).not.toContain('SaveSelection')
})
it('enables a non-Google observer the server qualified even without static processing', async () => {
  const qwen: FakeProvidersOptions = {
    ...models,
    models: [
      {
        providerId: 'p',
        modelId: 'qwen/vl',
        label: 'Qwen VL',
        vision: true,
        videoInput: true,
        inlineStaticVideo: false,
        stages: [Stage.OBSERVE],
      },
      models.models![1],
    ],
    selections: [
      { stage: Stage.OBSERVE, providerId: 'p', modelId: 'qwen/vl' },
      { stage: Stage.WRITE, providerId: 'p', modelId: 'w' },
    ],
  }
  const quotes: unknown[] = []
  mount(
    {
      quoteRequests: quotes,
      eligibility: [{ providerId: 'p', modelId: 'qwen/vl', status: 'eligible' }],
    },
    {},
    qwen,
  )
  await selectSource()
  await screen.findByRole('button', { name: '최대 20 크레딧 · 승인하고 생성' })
  expect(quotes[0]).toMatchObject({ observeModel: { providerId: 'p', modelId: 'qwen/vl' } })
})
it('greys every ineligible observer with its one reason and keeps the refused saved choice', async () => {
  const observers = [
    ['o', 'price_ceiling_unavailable', '요금 상한을 확인할 수 없는 경로예요'],
    ['absent', 'video_input_absent', '영상 입력을 받지 않는 모델이에요'],
    ['route', 'inline_endpoint_unavailable', '지금 클립 영상을 그대로 받을 수 있는 경로가 없어요'],
    [
      'params',
      'required_parameters_unsupported',
      '클립 분석 요청에 필요한 설정을 지원하지 않는 경로예요',
    ],
    ['fine', 'eligible', ''],
  ] as const
  const many: FakeProvidersOptions = {
    ...models,
    models: [
      ...observers.map(([id]) => ({
        providerId: 'p',
        modelId: id,
        label: `Observer ${id}`,
        vision: true,
        videoInput: id !== 'absent',
        stages: [Stage.OBSERVE],
      })),
      models.models![1],
    ],
  }
  const calls: string[] = []
  mount(
    {
      calls,
      eligibility: observers.map(([id, status]) => ({ providerId: 'p', modelId: id, status })),
    },
    {},
    many,
  )
  const user = userEvent.setup()
  const observe = await screen.findByRole('combobox', { name: /관찰 모델/ })
  await waitFor(() => expect(observe).toBeEnabled())
  // The refused saved choice is still the field's value, with its reason described.
  expect(observe).toHaveTextContent('Observer o')
  await waitFor(() =>
    expect(observe).toHaveAccessibleDescription(/요금 상한을 확인할 수 없는 경로예요/),
  )
  await user.click(observe)
  for (const [id, , reason] of observers) {
    const option = screen.getByRole('option', { name: new RegExp(`Observer ${id}`) })
    if (reason) {
      expect(option).toHaveAttribute('aria-disabled', 'true')
      expect(option).toHaveTextContent(reason)
    } else {
      expect(option).not.toHaveAttribute('aria-disabled')
    }
  }
  // Picking a refused model saves nothing; the eligible one is not chosen for the user.
  await user.click(screen.getByRole('option', { name: /Observer params/ }))
  expect(calls).not.toContain('SaveSelection')
  expect(observe).toHaveTextContent('Observer o')
})
it('fails the clip action closed while eligibility loads or cannot be read, and retries on request', async () => {
  let down = true
  const calls: string[] = []
  mount({ eligibilityFails: () => down, calls })
  await selectSource()
  expect(screen.getByRole('button', { name: '생성' })).toBeDisabled()
  await screen.findAllByText(/관찰 모델이 클립 분석에 쓸 수 있는지 확인하지 못했어요/)
  expect(calls).not.toContain('QuoteClipGeneration')
  expect(screen.queryByText(/NETWORK_UNAVAILABLE/)).not.toBeInTheDocument()
  down = false
  await userEvent.setup().click(screen.getByRole('button', { name: '다시 확인' }))
  await screen.findByRole('button', { name: '최대 20 크레딧 · 승인하고 생성' })
})
it('never treats an unspecified, unknown or duplicated status as eligible', async () => {
  for (const eligibility of [
    [{ providerId: 'p', modelId: 'o', status: 'unspecified' as const }],
    [
      { providerId: 'p', modelId: 'o', status: 'eligible' as const },
      { providerId: 'p', modelId: 'o', status: 'price_ceiling_unavailable' as const },
    ],
    [],
  ]) {
    const calls: string[] = []
    const view = mount({ eligibility, calls })
    await selectSource()
    expect(screen.getByRole('button', { name: '생성' })).toBeDisabled()
    expect(calls).not.toContain('QuoteClipGeneration')
    expect(
      screen.getAllByText(
        '선택한 관찰 모델은 클립 분석에 아직 쓸 수 없어요. 다른 모델을 선택해 주세요.',
      ).length,
    ).toBeGreaterThanOrEqual(1)
    view.unmount()
    vi.restoreAllMocks()
  }
})
it('does not automatically retry a failed start request', async () => {
  const calls: string[] = []
  mount({ calls, generationFails: true })
  const { user } = await selectSource()
  await user.click(await screen.findByRole('button', { name: '최대 20 크레딧 · 승인하고 생성' }))
  await screen.findByRole('alert')
  expect(calls.filter((c) => c === 'StartClipGeneration')).toHaveLength(1)
})

it('resolves a lost accepted response by owned identity reads without replaying the start', async () => {
  const starts: unknown[] = [],
    calls: string[] = []
  const job: FakeGenerationJobRow = {
    id: 'clip-job',
    kind: 'generate_clip',
    clipProjectId: 'clip',
    status: 'running',
    stage: 'analyze',
  }
  const view = mount(
    { generationAmbiguous: true, generationStarts: starts, calls },
    { jobs: [job] },
  )
  const { user, revoke } = await selectSource()
  await user.dblClick(await screen.findByRole('button', { name: '최대 20 크레딧 · 승인하고 생성' }))
  await screen.findByText('영상 분석')
  await waitFor(() => expect(screen.queryByText(/요청의 접수 여부를 확인/)).not.toBeInTheDocument())
  expect(starts).toHaveLength(1)
  expect(revoke).not.toHaveBeenCalled()
  expect(screen.getByLabelText('clip.mp4')).toHaveAttribute('src', 'blob:clip-source')
  expect(screen.getByLabelText(/^원본 영상 (다시 )?선택$/)).toBeDisabled()
  view.unmount()
  expect(revoke).toHaveBeenCalledExactlyOnceWith('blob:clip-source')
  expect(calls).not.toContain('DiscardClipSourceBatch')
})

it('does not attach an ambiguous selection to a different tab’s terminal job', async () => {
  const starts: unknown[] = []
  const other: FakeGenerationJobRow = {
    id: 'other-job',
    kind: 'generate_clip',
    clipProjectId: 'clip',
    status: 'failed',
    stage: 'prepare',
  }
  const view = mount(
    {
      generationFails: true,
      generationStarts: starts,
      readProject: (p) => ({
        ...p,
        latestJob: other,
        latestAttempt: { jobId: 'other-job', batchId: 'other-batch', quoteId: 'other-quote' },
      }),
    },
    { jobs: [other] },
  )
  const { user, revoke } = await selectSource()
  await user.click(await screen.findByRole('button', { name: '최대 20 크레딧 · 승인하고 생성' }))
  await screen.findByText(/요청의 접수 여부를 확인/)
  await user.click(screen.getByRole('button', { name: '접수된 작업 다시 확인' }))
  expect(starts).toHaveLength(1)
  expect(revoke).not.toHaveBeenCalled()
  expect(screen.getByLabelText('clip.mp4')).toBeInTheDocument()
  view.unmount()
})

it('invalidates a quote for dirty settings, even when reverted, without generating', async () => {
  const quotes: unknown[] = [],
    starts: unknown[] = []
  mount({ quoteRequests: quotes, generationStarts: starts })
  const { user } = await selectSource()
  await screen.findByRole('button', { name: '최대 20 크레딧 · 승인하고 생성' })
  expect(quotes).toHaveLength(1)
  const title = screen.getByLabelText('클립 제목')
  await user.type(title, 'x')
  expect(screen.queryByRole('button', { name: /승인하고 생성/ })).not.toBeInTheDocument()
  expect(quotes).toHaveLength(1)
  await user.keyboard('{Backspace}')
  await screen.findByRole('button', { name: '최대 20 크레딧 · 승인하고 생성' })
  expect(quotes).toHaveLength(2)
  expect(starts).toHaveLength(0)
})

it('refreshes the quote after a model change and never starts while that choice is saving', async () => {
  const quotes: unknown[] = [],
    starts: unknown[] = []
  mount(
    { quoteRequests: quotes, generationStarts: starts },
    {},
    {
      ...models,
      models: [
        ...models.models!,
        { providerId: 'p', modelId: 'w2', label: 'Writer two', stages: [Stage.WRITE] },
      ],
    },
  )
  const { user } = await selectSource()
  await screen.findByRole('button', { name: '최대 20 크레딧 · 승인하고 생성' })
  await user.click(screen.getByRole('combobox', { name: /작성/ }))
  await user.click(screen.getByRole('option', { name: 'Writer two' }))
  await waitFor(() => expect(quotes).toHaveLength(2))
  expect(quotes[1]).toMatchObject({ writeModel: { providerId: 'p', modelId: 'w2' } })
  expect(starts).toHaveLength(0)
})

it('finishes the locally owned job even when another tab becomes the latest project job', async () => {
  const own: FakeGenerationJobRow = {
    id: 'clip-job',
    kind: 'generate_clip',
    clipProjectId: 'clip',
    status: 'running',
    stage: 'analyze',
  }
  const other: FakeGenerationJobRow = {
    id: 'other-job',
    kind: 'generate_clip',
    clipProjectId: 'clip',
    status: 'running',
    stage: 'prepare',
  }
  let replaceLatest = false
  const view = mount(
    { readProject: (p) => (replaceLatest ? { ...p, latestJob: other } : p) },
    { jobs: [own, other] },
  )
  const { user, revoke } = await selectSource()
  await user.click(await screen.findByRole('button', { name: '최대 20 크레딧 · 승인하고 생성' }))
  await screen.findByText('영상 분석')
  replaceLatest = true
  await act(() =>
    view.queryClient.invalidateQueries({ queryKey: clipProjectsKey(view.transport, 'alice') }),
  )
  expect(revoke).not.toHaveBeenCalled()
  expect(screen.getByLabelText('clip.mp4')).toBeInTheDocument()
  own.status = 'failed'
  await act(() =>
    view.queryClient.refetchQueries({
      queryKey: createConnectQueryKey({
        schema: GenerationService.method.getGeneration,
        input: { id: own.id },
        transport: view.transport,
        cardinality: 'finite',
      }),
    }),
  )
  await waitFor(() =>
    expect(screen.getByLabelText('clip.mp4')).toHaveAttribute('src', 'blob:clip-source'),
  )
  expect(revoke).not.toHaveBeenCalled()
  expect(screen.getByLabelText(/^원본 영상 (다시 )?선택$/)).toBeDisabled()
})

it('replaces source-bound quotes and sends only the newly displayed quote', async () => {
  const quotes: unknown[] = [],
    starts: unknown[] = []
  mount({ quoteRequests: quotes, generationStarts: starts })
  await selectSource()
  await screen.findByRole('button', { name: '최대 20 크레딧 · 승인하고 생성' })
  const { user } = await selectSource()
  await waitFor(() => expect(quotes).toHaveLength(2))
  await user.click(await screen.findByRole('button', { name: '최대 20 크레딧 · 승인하고 생성' }))
  await waitFor(() => expect(starts).toHaveLength(1))
  expect(starts[0]).toMatchObject({
    quoteId: 'quote-2',
    approvedMaxCredits: 20,
    batchId: 'batch-2',
  })
})

it('expires approval and requires a refreshed displayed maximum and another explicit click', async () => {
  const starts: unknown[] = [],
    quotes: unknown[] = []
  const options: FakeClipsOptions = {
    quoteExpiresAt: new Date(Date.now() + 60_000).toISOString(),
    quoteRequests: quotes,
    generationStarts: starts,
  }
  mount(options)
  const { user } = await selectSource()
  await screen.findByRole('button', { name: '최대 20 크레딧 · 승인하고 생성' })
  vi.spyOn(Date, 'now').mockReturnValue(Date.now() + 61_000)
  await act(() => initializeI18n('en'))
  fireEvent(window, new Event('focus'))
  expect(
    await screen.findByText('This approval window expired. Check a new ceiling.'),
  ).toBeVisible()
  expect(starts).toHaveLength(0)
  await user.click(screen.getByRole('button', { name: 'Refresh maximum credits' }))
  await waitFor(() => expect(quotes).toHaveLength(2))
  expect(starts).toHaveLength(0)
})

it.each([false, true])(
  'shows affordability without clearing model selections (master=%s)',
  async (master) => {
    const starts: unknown[] = []
    mount({ quoteMaxCredits: 79, generationStarts: starts }, {}, models, {
      plan: master ? ProtoPlan.MASTER : ProtoPlan.FREE,
      balance: { credits: 12, unlimited: master, renewsAt: '2026-09-30T15:00:00Z' },
    })
    await selectSource()
    const button = await screen.findByRole('button', { name: '최대 79 크레딧 · 승인하고 생성' })
    if (master) {
      expect(button).toBeEnabled()
      expect(screen.getByText(/마스터는 크레딧을 차감하지/)).toBeVisible()
    } else {
      expect(button).toBeDisabled()
      expect(screen.getByText(/크레딧이 79 필요한데 12만 남았어요/)).toBeVisible()
    }
    expect(screen.getByRole('combobox', { name: /관찰/ })).toHaveTextContent('Video observer')
    expect(starts).toHaveLength(0)
  },
)

it('displays a pricing refusal beside generation while leaving the previous result downloadable', async () => {
  mount({ projects: [{ ...project, result }], quoteFails: 'CLIP_MODEL_PRICING_UNAVAILABLE' })
  // A saved result opens on ③, where the download is docked; the refusal belongs beside the
  // generation that earned it, on ①.
  expect(await screen.findByLabelText('클립 미리보기')).toHaveAttribute('src', result.viewUrl)
  expect(screen.getByRole('link', { name: '영상 다운로드' })).toBeEnabled()
  await goToStep('클립 생성')
  await selectSource()
  await screen.findByRole('alert')
  expect(screen.getByRole('button', { name: '생성' })).toBeDisabled()
})

it.each([0, 7])(
  'keeps terminal settlement pending until the authoritative %s-credit charge arrives',
  async (charge) => {
    const job: FakeGenerationJobRow = {
      id: 'settle-job',
      kind: 'generate_clip',
      clipProjectId: 'clip',
      status: 'failed',
      stage: 'analyze',
      failureReason: 'CLIP_PROCESSING_FAILED',
    }
    let accounting: ClipAccounting = {
      jobId: job.id,
      status: 'settling',
      approvedMaxCredits: 79,
      reservedCredits: 40,
      settled: false,
    }
    const view = mount(
      {
        projects: [{ ...project, result, latestJob: job }],
        readProject: (p) => ({ ...p, accounting }),
      },
      { jobs: [job] },
    )
    await screen.findByText('영상 처리는 끝났고 크레딧을 정산하는 중이에요.')
    const credit = within(screen.getByRole('region', { name: '이번 작업의 크레딧' }))
    expect(credit.getByText('79 크레딧')).toBeVisible()
    expect(credit.getByText('40 크레딧')).toBeVisible()
    expect(credit.getAllByText('확인 중')).toHaveLength(2)
    expect(credit.queryByText('0 크레딧')).not.toBeInTheDocument()
    accounting = {
      ...accounting,
      status: 'settled',
      finalChargeCredits: charge,
      refundCredits: 40 - charge,
      settled: true,
    }
    await act(() =>
      view.queryClient.invalidateQueries({ queryKey: clipProjectsKey(view.transport, 'alice') }),
    )
    await screen.findByText('크레딧 정산이 완료됐어요.')
    expect(credit.getByText(charge + ' 크레딧')).toBeVisible()
    expect(credit.getAllByText(40 - charge + ' 크레딧').length).toBeGreaterThan(0)
    // The settlement is reported on every step; the preserved result is on ③.
    await goToStep('클립 완성')
    expect(await screen.findByLabelText('클립 미리보기')).toHaveAttribute('src', result.viewUrl)
  },
)

it('releases previews immediately on logout and pagehide without discarding an accepted batch', async () => {
  const calls: string[] = []
  mount(
    { calls },
    {
      jobs: [
        {
          id: 'clip-job',
          kind: 'generate_clip',
          clipProjectId: 'clip',
          status: 'running',
          stage: 'prepare',
        },
      ],
    },
  )
  const { user, revoke } = await selectSource()
  await user.click(await screen.findByRole('button', { name: '최대 20 크레딧 · 승인하고 생성' }))
  await screen.findByRole('progressbar', { name: '원본 확인' })
  act(endSession)
  expect(revoke).toHaveBeenCalledExactlyOnceWith('blob:clip-source')
  expect(screen.queryByLabelText('clip.mp4')).not.toBeInTheDocument()
  fireEvent(window, new Event('pagehide'))
  expect(revoke).toHaveBeenCalledTimes(1)
  expect(calls).not.toContain('DiscardClipSourceBatch')
})
