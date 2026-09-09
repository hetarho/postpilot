import { afterEach, expect, it, vi } from 'vitest'
import { act, fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createConnectQueryKey } from '@connectrpc/connect-query'
import { initializeI18n } from '@/app/providers/i18n'
import { readSourceManifest } from '@/features/upload-clip-sources'
import { putBlobWithProgress } from '@/shared/lib/upload'
import { GenerationService, Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import type { FakeClipProject, FakeClipsOptions } from '@/test/clips'
import type { FakeGenerationJobRow, FakeJobsOptions } from '@/test/jobs'
import type { FakeProvidersOptions } from '@/test/providers'

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
) {
  return renderAppAt('/clips/clip', {
    user: { id: 'alice' },
    providers,
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
        },
      ],
      projects: [project],
      ...clips,
    },
  })
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
  const input = await screen.findByLabelText('원본 영상 선택')
  await waitFor(() => expect(input).toBeEnabled())
  const user = userEvent.setup()
  await user.upload(input, file)
  await screen.findByText('업로드 확인 완료')
  return { user, revoke }
}
it('starts once, releases local bytes, locks setup, then refetches the finished result', async () => {
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
  const generate = screen.getByRole('button', { name: '생성' })
  await waitFor(() => expect(generate).toBeEnabled())
  await user.dblClick(generate)
  await screen.findByText('영상 분석')
  expect(starts).toHaveLength(1)
  expect(starts[0]).toMatchObject({
    projectId: 'clip',
    observeModel: { providerId: 'p', modelId: 'o' },
    writeModel: { providerId: 'p', modelId: 'w' },
  })
  expect(revoke).toHaveBeenCalledWith('blob:clip-source')
  expect(screen.queryByText('clip.mp4')).not.toBeInTheDocument()
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
  const video = await screen.findByLabelText('클립 미리보기')
  expect(video).toHaveAttribute('src', result.viewUrl)
  expect(video).toHaveAttribute('controls')
  expect(video).toHaveAttribute('preload', 'metadata')
  expect(screen.getByRole('link', { name: '영상 다운로드' })).toHaveAttribute(
    'href',
    result.downloadUrl,
  )
  expect(calls.filter((c) => c === 'GetClipProject').length).toBeGreaterThan(1)
  expect(calls).not.toContain('DiscardClipSourceBatch')
  // A new selection after completion must not be mistaken for the consumed batch.
  await selectSource()
  expect(screen.getByRole('button', { name: '다시 생성' })).toBeEnabled()
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
  expect(await screen.findByLabelText('클립 미리보기')).toHaveAttribute('src', result.viewUrl)
  expect(await screen.findByText(/크레딧이 79 필요한데 12만 남았어요/)).toBeInTheDocument()
  expect(screen.getByText('원본 확인 단계에서 실패했어요')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: '크레딧·요금제 확인' })).toHaveAttribute('href', '/plans')
  expect(screen.getByRole('button', { name: '다시 생성' })).toBeDisabled()
  await selectSource()
  expect(screen.getByRole('button', { name: '다시 생성' })).toBeEnabled()
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
it('blocks non-video observers and never substitutes another registered model', async () => {
  const nonVideo: FakeProvidersOptions = {
    ...models,
    models: models.models!.map((m) => ({ ...m, videoInput: false })),
  }
  const starts: unknown[] = []
  mount({ generationStarts: starts }, {}, nonVideo)
  await selectSource()
  expect(screen.getByRole('button', { name: '생성' })).toBeDisabled()
  expect(starts).toHaveLength(0)
  expect(screen.getByText('영상 입력을 지원하는 관찰 모델을 선택해 주세요.')).toBeInTheDocument()
})
it('does not automatically retry a failed start request', async () => {
  const calls: string[] = []
  mount({ calls, generationFails: true })
  const { user } = await selectSource()
  await user.click(screen.getByRole('button', { name: '생성' }))
  await screen.findByRole('alert')
  expect(calls.filter((c) => c === 'StartClipGeneration')).toHaveLength(1)
})
