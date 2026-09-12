import { act, fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it } from 'vitest'
import { Code } from '@connectrpc/connect'
import { initializeI18n } from '@/app/providers/i18n'
import { clipProjectsKey } from '@/entities/clip-project'
import { renderAppAt } from '@/test/app'
import { clipTimelineFixture } from '@/test/clip-editing'
import { connectAppError } from '@/test/app-error'
import type { FakeClipProject, FakeClipsOptions } from '@/test/clips'
import type { FakeGenerationJobRow } from '@/test/jobs'

afterEach(() => initializeI18n('ko'))
const project = (): FakeClipProject => ({
  id: 'clip',
  title: 'Lifecycle',
  videoTemplateId: '',
  ratio: 'vertical',
  targetDurationMs: 19800,
  answers: [],
  disclosure: '',
  cta: '',
  editPlanRevision: 1,
  renderedPlanRevision: 1,
  editing: clipTimelineFixture(),
  result: {
    id: 'render-1',
    contentType: 'video/mp4',
    bytes: 5,
    durationMs: 19800,
    createdAt: '2026-09-13T00:00:00Z',
    viewUrl: 'https://private.test/result',
    downloadUrl: 'https://private.test/download',
  },
})
const mount = (options: FakeClipsOptions = {}, jobs: FakeGenerationJobRow[] = []) =>
  renderAppAt('/clips/clip', {
    user: { id: 'alice' },
    clips: { projects: [project()], ...options },
    jobs: { jobs },
  })
const confirm = () => screen.findByRole('button', { name: '확정하기' })

it('downloads without confirming, guards the result tab and confirms only once on double click', async () => {
  const calls: string[] = []
  mount({ calls })
  await confirm()
  expect(screen.getByRole('link', { name: '렌더 1 다운로드' })).toHaveAttribute(
    'href',
    project().result!.downloadUrl,
  )
  expect(calls).not.toContain('FinalizeClipProject')
  await userEvent.click(screen.getByRole('tab', { name: '클립 완성' }))
  expect(screen.getByText(/수정 단계에서 확정하기를/)).toBeVisible()
  expect(screen.queryByLabelText('클립 미리보기')).not.toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: '클립 다듬기로 이동' }))
  await userEvent.dblClick(await confirm())
  await waitFor(() =>
    expect(screen.queryByRole('tablist', { name: '클립 단계' })).not.toBeInTheDocument(),
  )
  expect(screen.getByLabelText('클립 미리보기')).toHaveAttribute('src', project().result!.viewUrl)
  expect(screen.getByRole('link', { name: '영상 다운로드' })).toBeVisible()
  expect(screen.queryByLabelText('편집 타임라인')).not.toBeInTheDocument()
  expect(calls.filter((c) => c === 'FinalizeClipProject')).toHaveLength(1)
})

it('flushes the current plan before confirming and refuses the now-stale render', async () => {
  const calls: string[] = [],
    writes: NonNullable<FakeClipsOptions['planWrites']> = []
  mount({ calls, planWrites: writes })
  await confirm()
  fireEvent.change(screen.getByLabelText('원본 시작 (초)'), { target: { value: '0.5' } })
  await userEvent.click(await confirm())
  await waitFor(() => expect(writes).toHaveLength(1))
  await waitFor(() => expect(screen.getByLabelText('원본 시작 (초)')).toHaveValue(0.5))
  expect(calls).not.toContain('FinalizeClipProject')
  expect(screen.getByRole('link', { name: '렌더 1 다운로드' })).toBeInTheDocument()
  expect(screen.getByRole('tab', { name: '클립 다듬기' })).toHaveAttribute('aria-selected', 'true')
})

it('keeps the draft when its save conflicts and never sends confirmation', async () => {
  const calls: string[] = []
  mount({ calls, planSaveConflict: true })
  await confirm()
  fireEvent.change(screen.getByLabelText('원본 시작 (초)'), { target: { value: '0.5' } })
  await userEvent.click(await confirm())
  await screen.findByRole('button', { name: '내 편집을 최신 버전에 적용' })
  expect(screen.getByLabelText('원본 시작 (초)')).toHaveValue(0.5)
  expect(calls).not.toContain('FinalizeClipProject')
})

it('resolves a lost confirmation reply by reading the owned project and reopens result-only', async () => {
  let saved: FakeClipProject | undefined
  const calls: string[] = []
  const view = mount({
    calls,
    finalize: async (_, p) => {
      p.finalized = { at: '2026-09-13T00:00:00Z', planRevision: 1, resultId: 'render-1' }
      p.editing = undefined
      saved = structuredClone(p)
      throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    },
  })
  await userEvent.click(await confirm())
  await screen.findByRole('link', { name: '영상 다운로드' })
  expect(calls.filter((c) => c === 'FinalizeClipProject')).toHaveLength(1)
  view.unmount()
  const reads: string[] = []
  mount({ projects: [saved!], calls: reads })
  await screen.findByRole('link', { name: '영상 다운로드' })
  expect(screen.queryByRole('tablist', { name: '클립 단계' })).not.toBeInTheDocument()
  expect(reads).not.toContain('GetClipSources')
})

it.each(['ko', 'en'] as const)(
  'focuses running work, cancels once and restores the previous result (%s)',
  async (lang) => {
    initializeI18n(lang)
    const job: FakeGenerationJobRow = {
      id: 'job',
      kind: 'generate_clip',
      status: 'running',
      stage: 'analyze',
      clipProjectId: 'clip',
      progressDone: 1,
      progressTotal: 3,
      canCancel: true,
      cancellationPolicyVersion: 1,
    }
    let cancelled = false
    const calls: string[] = []
    mount(
      {
        calls,
        projects: [{ ...project(), latestJob: job }],
        readProject: (p) => (cancelled ? { ...p, latestJob: { ...job, status: 'cancelled' } } : p),
        cancel: async () => {
          cancelled = true
          job.status = 'cancelled'
          job.cancelRequestedAt = '2026-09-13T00:00:00Z'
          return job
        },
      },
      [job],
    )
    const cancel = await screen.findByRole('button', { name: lang === 'ko' ? '취소' : 'Cancel' })
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('편집 타임라인')).not.toBeInTheDocument()
    expect(screen.getAllByRole('progressbar')).toHaveLength(1)
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '1')
    expect(screen.getByText(/50%/)).toBeVisible()
    await userEvent.dblClick(cancel)
    await screen.findByRole('link', {
      name: lang === 'ko' ? '렌더 1 다운로드' : 'Download render 1',
    })
    expect(calls.filter((c) => c === 'CancelClipJob')).toHaveLength(1)
    expect(calls).not.toContain('StartClipGeneration')
  },
)

it('returning to the list never cancels work; opening it again restores focused progress', async () => {
  const job: FakeGenerationJobRow = {
    id: 'job',
    kind: 'render_clip',
    status: 'running',
    stage: 'render',
    clipProjectId: 'clip',
    canCancel: true,
  }
  const calls: string[] = []
  const view = mount({ projects: [{ ...project(), latestJob: job }], calls }, [job])
  await screen.findByRole('progressbar')
  await userEvent.click(screen.getByRole('link', { name: '클립 목록' }))
  await screen.findByRole('link', { name: /Lifecycle/ })
  await userEvent.click(screen.getByRole('link', { name: /Lifecycle/ }))
  await screen.findByRole('progressbar')
  expect(calls).not.toContain('CancelClipJob')
  await act(() =>
    view.queryClient.invalidateQueries({ queryKey: clipProjectsKey(view.transport, 'alice') }),
  )
  expect(screen.queryByRole('tablist', { name: '클립 단계' })).not.toBeInTheDocument()
})

it('keeps cancellation unavailable for a legacy attempt', async () => {
  const job = {
    id: 'legacy',
    kind: 'generate_clip',
    status: 'running',
    stage: 'plan',
    canCancel: false,
    cancellationPolicyVersion: 0,
  }
  mount({ projects: [{ ...project(), latestJob: job }] }, [job])
  expect(await screen.findByRole('button', { name: '취소' })).toBeDisabled()
  expect(screen.getByText(/이 작업은 이전 요금 정책/)).toBeVisible()
})

it.each([false, true])(
  'separates confirmed AI use, cancellation addition, debit and refund (exempt=%s)',
  async (exempt) => {
    const job = { id: 'cancelled', kind: 'generate_clip', status: 'cancelled', stage: 'render' }
    mount(
      {
        projects: [
          {
            ...project(),
            latestJob: job,
            accounting: {
              jobId: job.id,
              status: exempt ? 'exempt' : 'settled',
              settled: true,
              approvedMaxCredits: 21,
              reservedCredits: exempt ? 0 : 21,
              confirmedChargeCredits: exempt ? 0 : 4,
              cancellationFeeCredits: exempt ? 0 : 9,
              finalChargeCredits: exempt ? 0 : 13,
              refundCredits: exempt ? 0 : 8,
              ...(exempt
                ? {
                    shadowConfirmedChargeCredits: 4,
                    shadowCancellationFeeCredits: 9,
                    shadowChargeCredits: 13,
                  }
                : {}),
            },
          },
        ],
      },
      [job],
    )
    await confirm()
    expect(screen.getByText('확인된 AI 사용분')).toBeVisible()
    expect(screen.getByText('취소 추가분')).toBeVisible()
    expect(screen.getByText('13 크레딧')).toBeVisible()
    if (exempt) expect(screen.getByText('참고 총액 (차감 없음)')).toBeVisible()
    else expect(screen.getByText('8 크레딧')).toBeVisible()
  },
)
