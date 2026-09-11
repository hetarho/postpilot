import { afterEach, expect, it, vi } from 'vitest'
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createConnectQueryKey } from '@connectrpc/connect-query'
import { initializeI18n } from '@/app/providers/i18n'
import { readSourceManifest } from '@/features/upload-clip-sources'
import { putBlobWithProgress } from '@/shared/lib/upload'
import { GenerationService } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { clipEditingFixture } from '@/test/clip-editing'
import type { FakeClipProject, FakeClipsOptions } from '@/test/clips'
import type { FakeGenerationJobRow, FakeJobsOptions } from '@/test/jobs'

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

function fixture(): FakeClipProject {
  return {
    id: 'clip',
    title: '제주',
    videoTemplateId: 'template',
    ratio: 'vertical',
    targetDurationMs: 19800,
    answers: [],
    disclosure: 'ad',
    cta: '',
    editPlanRevision: 1,
    renderedPlanRevision: 1,
    editing: clipEditingFixture(),
    result: {
      contentType: 'video/mp4',
      bytes: 5,
      durationMs: 19800,
      createdAt: '2026-09-10T00:00:00Z',
      viewUrl: 'https://private.test/old',
      downloadUrl: 'https://private.test/download',
    },
  }
}
async function mount(clips: FakeClipsOptions = {}, jobs: FakeJobsOptions = {}) {
  const view = renderAppAt('/clips/clip', {
    user: { id: 'alice' },
    providers: { models: [] },
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
      projects: [fixture()],
      ...clips,
    },
  })
  // A rendered result describes the project as 완성, so the workspace opens on ③; the correction
  // is step ② now, reached from the step bar rather than a 수정 button (CLIP-36).
  await screen.findByLabelText('클립 미리보기')
  await goToStep('클립 다듬기')
  await screen.findByRole('heading', { name: '컷·자막 수정' })
  return view
}
async function goToStep(name: '클립 생성' | '클립 다듬기' | '클립 완성') {
  await userEvent.click(await screen.findByRole('tab', { name }))
}
/** The retained result, checked where it now lives (③): a correction may neither lose it nor
 *  remint its presigned URL. Leaves the caller back on ②. */
async function expectResultKept() {
  await goToStep('클립 완성')
  expect(await screen.findByLabelText('클립 미리보기')).toHaveAttribute(
    'src',
    'https://private.test/old',
  )
  await goToStep('클립 다듬기')
}
it("docks exactly one committing control at a time, the current step's", async () => {
  await mount()
  // ② — the correction's own bar. ①'s approval and ③'s download belong to other panels.
  expect(screen.getByRole('button', { name: '다시 출력 · 크레딧 사용 없음' })).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: /승인하고 생성|^생성$/ })).not.toBeInTheDocument()
  expect(screen.queryByRole('link', { name: '영상 다운로드' })).not.toBeInTheDocument()

  await goToStep('클립 완성')
  expect(await screen.findByRole('link', { name: '영상 다운로드' })).toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: '다시 출력 · 크레딧 사용 없음' }),
  ).not.toBeInTheDocument()

  await goToStep('클립 생성')
  expect(await screen.findByRole('button', { name: /생성/ })).toBeInTheDocument()
  expect(screen.queryByRole('link', { name: '영상 다운로드' })).not.toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: '다시 출력 · 크레딧 사용 없음' }),
  ).not.toBeInTheDocument()
})
const cut = (number = 1) => within(screen.getByRole('region', { name: `컷 ${number}` }))
const change = (label: string, value: string, number = 1) =>
  fireEvent.change(cut(number).getByLabelText(label), { target: { value } })
async function select(ids = ['a', 'b']) {
  const files = ids.map((id) => new File(['clip'], `source-${id}.mp4`, { type: 'video/mp4' }))
  vi.mocked(readSourceManifest).mockResolvedValue(
    files.map((f, i) => ({
      filename: f.name,
      contentType: f.type,
      bytes: f.size,
      width: 1920,
      height: 1080,
      durationMs: 40000,
      fingerprint: ids[i]!.repeat(64),
    })),
  )
  vi.mocked(putBlobWithProgress).mockResolvedValue()
  vi.spyOn(URL, 'createObjectURL').mockImplementation((blob) => `blob:${(blob as File).name}`)
  const revoke = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
  const input = screen.getByLabelText('원본 영상 선택')
  await waitFor(() => expect(input).toBeEnabled())
  await userEvent.upload(input, files)
  return revoke
}
it('keeps the result mounted across every edit and saves exact fields with its revision', async () => {
  const writes: NonNullable<FakeClipsOptions['planWrites']> = []
  await mount({ planWrites: writes })
  change('원본 시작 (ms)', '1000')
  change('원본 끝 (ms)', '16000')
  change('컷 길이 (ms)', '16000')
  change('자막 원문', '정확한 한국어 <copy>')
  change('자막 시작 (컷 내 ms)', '200')
  change('자막 끝 (컷 내 ms)', '2000')
  change('원본 소리 (%)', '25')
  await userEvent.click(cut().getByRole('combobox', { name: /자막 위치/ }))
  await userEvent.click(screen.getByRole('option', { name: '하단' }))
  await userEvent.click(cut().getByRole('combobox', { name: /가로 정렬/ }))
  await userEvent.click(screen.getByRole('option', { name: '왼쪽' }))
  await userEvent.click(cut().getByRole('combobox', { name: /자막 스타일/ }))
  await userEvent.click(screen.getByRole('option', { name: '메모' }))
  await userEvent.click(cut().getByRole('combobox', { name: /강조 색상/ }))
  await userEvent.click(screen.getByRole('option', { name: '청록' }))
  await userEvent.click(screen.getAllByRole('button', { name: '아래로 이동' })[0]!)
  expect(cut(2).getByLabelText('자막 원문')).toHaveValue('정확한 한국어 <copy>')
  await userEvent.click(screen.getByRole('button', { name: '수정 저장' }))
  await screen.findByText('수정됨 · 다시 출력 필요')
  expect(writes).toHaveLength(1)
  expect(writes[0]).toMatchObject({
    revision: 1,
    plan: {
      durationMs: 25800,
      cuts: [
        { id: 'cut-b' },
        {
          id: 'cut-a',
          startMs: 1000,
          endMs: 17000,
          volumePermille: 250,
          copy: {
            text: '정확한 한국어 <copy>',
            style: 'memo',
            // The anchor the wire calls `position`; it round-trips through the
            // proto mapper on the way back, so a lost mapping fails here. 메모
            // sits LEFT at the top or the bottom (CDS-24).
            anchor: 'bottom',
            align: 'left',
            accent: 'teal',
            startMs: 200,
            endMs: 2000,
          },
        },
      ],
    },
  })
  await expectResultKept()
  expect(screen.getByRole('button', { name: '다시 출력 · 크레딧 사용 없음' })).toBeDisabled()
})
it('shows bounds beside fields and never sends an invalid plan', async () => {
  const writes: NonNullable<FakeClipsOptions['planWrites']> = []
  await mount({ planWrites: writes })
  change('원본 끝 (ms)', '50000')
  expect(cut().getByLabelText('원본 끝 (ms)')).toHaveAttribute('aria-invalid', 'true')
  expect(cut().getByText(/원본의 0~40000/)).toBeVisible()
  expect(screen.getByRole('button', { name: '수정 저장' })).toBeDisabled()
  change('원본 끝 (ms)', '1000')
  expect(screen.getByText(/최종 길이는/)).toBeVisible()
  change('자막 끝 (컷 내 ms)', '2000')
  expect(cut().getByText('자막은 컷 안에서 시작보다 끝이 늦어야 해요.')).toBeVisible()
  change('원본 소리 (%)', '101')
  expect(cut().getByText('볼륨은 0~100%로 입력해 주세요.')).toBeVisible()
  await userEvent.click(screen.getByRole('button', { name: '수정 저장' }))
  expect(writes).toEqual([])
})
it('keeps local edits and video on optimistic conflict, and guards leaving', async () => {
  const { router } = await mount({ planSaveConflict: true })
  change('자막 원문', '잃지 않을 수정')
  await userEvent.click(screen.getByRole('button', { name: '수정 저장' }))
  await screen.findByText(/저장된 수정본이 변경되었어요/)
  expect(cut().getByLabelText('자막 원문')).toHaveValue('잃지 않을 수정')
  await expectResultKept()
  expect(cut().getByLabelText('자막 원문')).toHaveValue('잃지 않을 수정')
  // The guard is the PAGE's now, so it still fires after the owner has looked at another step.
  await goToStep('클립 완성')
  await userEvent.click(screen.getByRole('link', { name: '클립 목록' }))
  await screen.findByRole('dialog')
  expect(router.state.location.pathname).toBe('/clips/clip')
  await userEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: '취소' }))
  // And the draft is still there after the whole round trip through the other steps.
  await goToStep('클립 다듬기')
  expect(cut().getByLabelText('자막 원문')).toHaveValue('잃지 않을 수정')
})
it('checks every required fingerprint before reserving any batch', async () => {
  const calls: string[] = []
  await mount({ calls })
  await select(['a'])
  await screen.findByText('아직 필요한 원본: source-b.mp4')
  expect(calls).not.toContain('CreateClipSourceBatch')
  await select(['x'])
  await screen.findByText('일치하지 않거나 필요하지 않은 파일: source-x.mp4')
  expect(calls).not.toContain('CreateClipSourceBatch')
})
it('deletes a cut, reselects only its remaining source and rerenders once with no AI models', async () => {
  const calls: string[] = [],
    starts: unknown[] = []
  const job: FakeGenerationJobRow = {
    id: 'clip-render-job',
    kind: 'render_clip',
    clipProjectId: 'clip',
    status: 'running',
    stage: 'render',
  }
  let finished = false
  const view = await mount(
    {
      calls,
      renderStarts: starts,
      readProject: (p) =>
        finished
          ? {
              ...p,
              latestJob: job,
              renderedPlanRevision: p.editPlanRevision,
              result: {
                ...p.result!,
                createdAt: '2026-09-10T01:00:00Z',
                viewUrl: 'https://private.test/new',
              },
            }
          : p,
    },
    {
      jobs: [job],
      onRead: (j) => {
        finished = j.status === 'done'
      },
    },
  )
  change('원본 끝 (ms)', '20000')
  await userEvent.click(cut(2).getByRole('button', { name: '컷 삭제' }))
  await userEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: '컷 삭제' }))
  await userEvent.click(screen.getByRole('button', { name: '수정 저장' }))
  await screen.findByText('수정됨 · 다시 출력 필요')
  expect(screen.queryByText('source-b.mp4')).not.toBeInTheDocument()
  const revoke = await select(['a'])
  await screen.findByText('업로드 확인 완료')
  await userEvent.click(cut().getByRole('button', { name: '이 구간 원본 확인' }))
  expect(screen.getByLabelText('선택한 컷의 원본 미리보기')).toHaveAttribute(
    'src',
    'blob:source-a.mp4',
  )
  const button = screen.getByRole('button', { name: '다시 출력 · 크레딧 사용 없음' })
  fireEvent.click(button)
  fireEvent.click(button)
  await waitFor(() => expect(starts).toHaveLength(1))
  expect(calls).not.toContain('StartClipGeneration')
  await screen.findByRole('progressbar', { name: '영상 렌더링' })
  expect(revoke).not.toHaveBeenCalled()
  expect(cut().getByLabelText('자막 원문')).toBeDisabled()
  await expectResultKept()
  job.status = 'done'
  job.stage = 'cleanup'
  await act(() =>
    view.queryClient.refetchQueries({
      queryKey: createConnectQueryKey({
        schema: GenerationService.method.getGeneration,
        input: { id: job.id },
        transport: view.transport,
        cardinality: 'finite',
      }),
    }),
  )
  await waitFor(() =>
    expect(screen.getByLabelText('클립 미리보기')).toHaveAttribute(
      'src',
      'https://private.test/new',
    ),
  )
  expect(screen.queryByLabelText('선택한 컷의 원본 미리보기')).not.toBeInTheDocument()
  expect(revoke).toHaveBeenCalledExactlyOnceWith('blob:source-a.mp4')
  expect(
    JSON.stringify(
      view.queryClient
        .getQueryCache()
        .getAll()
        .map((q) => q.state.data),
    ),
  ).not.toContain('blob:')
})
it('allows correction after template deletion and leaves saved edits without a warning', async () => {
  const p = fixture()
  p.videoTemplateId = ''
  const { router } = await mount({ projects: [p], templates: [] })
  change('자막 원문', '저장한 수정')
  await userEvent.click(screen.getByRole('button', { name: '수정 저장' }))
  await screen.findByText('수정됨 · 다시 출력 필요')
  await userEvent.click(screen.getByRole('link', { name: '클립 목록' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/clips'))
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})
it('preserves saved corrections and the prior result after a failed free render', async () => {
  const calls: string[] = []
  const job: FakeGenerationJobRow = {
    id: 'clip-render-job',
    kind: 'render_clip',
    clipProjectId: 'clip',
    status: 'failed',
    stage: 'render',
    failureReason: 'CLIP_PROCESSING_FAILED',
  }
  await mount({ calls }, { jobs: [job] })
  change('자막 원문', '출력 실패에도 보존')
  await userEvent.click(screen.getByRole('button', { name: '수정 저장' }))
  await screen.findByText('수정됨 · 다시 출력 필요')
  const revoke = await select()
  await waitFor(() => expect(screen.getAllByText('업로드 확인 완료')).toHaveLength(2))
  await userEvent.click(screen.getByRole('button', { name: '다시 출력 · 크레딧 사용 없음' }))
  await waitFor(() => expect(screen.getByRole('alert')).toBeVisible())
  expect(cut().getByLabelText('자막 원문')).toHaveValue('출력 실패에도 보존')
  await expectResultKept()
  expect(screen.getByRole('button', { name: '수정 저장' })).toBeDisabled()
  expect(screen.getByRole('button', { name: '다시 출력 · 크레딧 사용 없음' })).toBeDisabled()
  expect(calls.filter((c) => c === 'StartClipRender')).toHaveLength(1)
  expect(calls).not.toContain('StartClipGeneration')
  expect(revoke).toHaveBeenCalledWith('blob:source-a.mp4')
  expect(revoke).toHaveBeenCalledWith('blob:source-b.mp4')
})
it('releases local previews on leave and leaves remote cleanup to the durable lease', async () => {
  const calls: string[] = []
  const { router } = await mount({ calls })
  const revoke = await select()
  await waitFor(() => expect(screen.getAllByText('업로드 확인 완료')).toHaveLength(2))
  await userEvent.click(screen.getByRole('link', { name: '클립 목록' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/clips'))
  expect(calls).not.toContain('DiscardClipSourceBatch')
  expect(revoke).toHaveBeenCalledWith('blob:source-a.mp4')
  expect(revoke).toHaveBeenCalledWith('blob:source-b.mp4')
})
it('explicitly discards the batch and local previews when source selection is cancelled', async () => {
  const calls: string[] = []
  await mount({ calls })
  const revoke = await select()
  await waitFor(() => expect(screen.getAllByText('업로드 확인 완료')).toHaveLength(2))
  await userEvent.click(screen.getByRole('button', { name: '선택 취소' }))
  await waitFor(() => expect(calls).toContain('DiscardClipSourceBatch'))
  expect(revoke).toHaveBeenCalledWith('blob:source-a.mp4')
  expect(revoke).toHaveBeenCalledWith('blob:source-b.mp4')
  expect(screen.getByRole('button', { name: '다시 출력 · 크레딧 사용 없음' })).toBeDisabled()
})
it('localizes the credit-free correction action and accessible field labels in English', async () => {
  await mount()
  await act(async () => {
    initializeI18n('en')
  })
  expect(screen.getByRole('button', { name: 'Rerender · no credits' })).toBeDisabled()
  expect(screen.getByRole('heading', { name: 'Edit cuts and captions' })).toBeVisible()
  expect(screen.getAllByLabelText('Original audio (%)')).toHaveLength(2)
})
