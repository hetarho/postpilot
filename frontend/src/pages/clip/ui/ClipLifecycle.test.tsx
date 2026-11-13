import { act, fireEvent, screen, waitFor, within } from '@testing-library/react'
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
  disclosure: '',
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
// Item selection retains the preview; the phone opens details explicitly.
const selectCut = async () => {
  await userEvent.click(
    within(screen.getByRole('region', { name: '장면 블록' })).getByRole('button', {
      name: '컷 1 편집',
    }),
  )
}

// CLIP-75, CLIP-76: 확정하기 in ② confirms at once and opens ③, where 완료로 표시 is the one
// irreversible step behind its dialog; a download confirms nothing.
it('downloads without confirming, confirms in ② and finalizes once in ③ on double click', async () => {
  const calls: string[] = []
  mount({ calls })
  await confirm()
  // ② carries no download: the render and its download stand in ③ (CLIP-149).
  expect(screen.queryByRole('link', { name: '렌더 1 다운로드' })).not.toBeInTheDocument()
  await userEvent.click(screen.getByRole('tab', { name: '완성' }))
  expect(screen.getByText(/확정하기를 누르면 이 단계에서 렌더해요/)).toBeVisible()
  expect(screen.getByRole('link', { name: '렌더 1 다운로드' })).toHaveAttribute(
    'href',
    project().result!.downloadUrl,
  )
  expect(calls).not.toContain('FinalizeClipProject')
  await userEvent.click(screen.getByRole('button', { name: '영상 다듬기로 가기' }))
  await userEvent.click(await confirm())
  await waitFor(() =>
    expect(screen.getByRole('tab', { name: '완성' })).toHaveAttribute('aria-selected', 'true'),
  )
  expect(calls.filter((c) => c === 'ConfirmClipProject')).toHaveLength(1)
  expect(screen.queryByText(/확정하기를 누르면 이 단계에서 렌더해요/)).not.toBeInTheDocument()
  expect(calls).not.toContain('FinalizeClipProject')
  await userEvent.dblClick(screen.getByRole('button', { name: '완료로 표시' }))
  const dialog = within(await screen.findByRole('dialog', { name: '클립을 완료로 표시할까요?' }))
  expect(calls).not.toContain('FinalizeClipProject')
  await userEvent.dblClick(dialog.getByRole('button', { name: '완료로 표시' }))
  await screen.findByRole('link', { name: /렌더 \d+ 다운로드/ })
  expect(screen.getByLabelText('클립 미리보기')).toHaveAttribute('src', project().result!.viewUrl)
  expect(screen.queryByRole('region', { name: '장면 블록' })).not.toBeInTheDocument()
  expect(calls.filter((c) => c === 'FinalizeClipProject')).toHaveLength(1)
})

it('flushes the current plan before confirming it and refuses to finalize the now-stale render', async () => {
  const calls: string[] = [],
    writes: NonNullable<FakeClipsOptions['planWrites']> = [],
    confirmations: Array<{ expectedPlanRevision: number }> = []
  mount({ calls, planWrites: writes, confirmations })
  await confirm()
  await selectCut()
  fireEvent.change(screen.getByLabelText('원본 시작 (초)'), { target: { value: '0.5' } })
  await userEvent.click(await confirm())
  await waitFor(() => expect(confirmations).toHaveLength(1))
  expect(writes).toHaveLength(1)
  expect(confirmations[0].expectedPlanRevision).toBe(2)
  expect(calls).not.toContain('FinalizeClipProject')
  expect(
    await screen.findByText('렌더하기로 현재 편집안을 출력한 뒤 완료로 표시해 주세요.'),
  ).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '완료로 표시' })).toBeDisabled()
})

it('keeps the draft when its save conflicts and never sends confirmation', async () => {
  const calls: string[] = []
  mount({ calls, planSaveConflict: true })
  await confirm()
  await selectCut()
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
  await userEvent.click(await screen.findByRole('button', { name: '완료로 표시' }))
  await userEvent.click(
    within(await screen.findByRole('dialog', { name: '클립을 완료로 표시할까요?' })).getByRole(
      'button',
      { name: '완료로 표시' },
    ),
  )
  await screen.findByRole('link', { name: /렌더 \d+ 다운로드/ })
  expect(calls.filter((c) => c === 'FinalizeClipProject')).toHaveLength(1)
  view.unmount()
  const reads: string[] = []
  mount({ projects: [saved!], calls: reads })
  await screen.findByRole('link', { name: /렌더 \d+ 다운로드/ })
  expect(screen.getByRole('tab', { name: '완성' })).toHaveAttribute('aria-selected', 'true')
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
    expect(screen.queryByRole('region', { name: '장면 블록' })).not.toBeInTheDocument()
    expect(screen.getAllByRole('progressbar')).toHaveLength(1)
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '1')
    expect(
      screen.getByText(lang === 'ko' ? /추가 취소 수수료는 없어요/ : /no extra cancellation fee/),
    ).toBeVisible()
    await userEvent.dblClick(cancel)
    expect(calls).not.toContain('CancelClipJob')
    const dialog = await screen.findByRole('dialog')
    expect(
      within(dialog).getByText(
        lang === 'ko' ? /추가 취소 수수료는 없어요/ : /no extra cancellation fee/,
      ),
    ).toBeVisible()
    await userEvent.click(
      within(dialog).getByRole('button', { name: lang === 'ko' ? '계속 제작' : 'Keep going' }),
    )
    expect(calls).not.toContain('CancelClipJob')
    await userEvent.click(cancel)
    await userEvent.dblClick(
      within(await screen.findByRole('dialog')).getByRole('button', {
        name: lang === 'ko' ? '제작 취소' : 'Cancel production',
      }),
    )
    // The previous result stands in ③ again, where its download is (CLIP-149).
    await userEvent.click(
      await screen.findByRole('tab', { name: lang === 'ko' ? '완성' : 'Finish' }),
    )
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
  await userEvent.click(screen.getByRole('link', { name: '네이버 클립으로 돌아가기' }))
  await screen.findByRole('link', { name: /Lifecycle/ })
  await userEvent.click(screen.getByRole('link', { name: /Lifecycle/ }))
  await screen.findByRole('progressbar')
  expect(calls).not.toContain('CancelClipJob')
  await act(() =>
    view.queryClient.invalidateQueries({ queryKey: clipProjectsKey(view.transport, 'alice') }),
  )
  expect(screen.queryByRole('tablist', { name: '클립 단계' })).not.toBeInTheDocument()
})

// CLIP-234: a finished storyline run lands on ③ even on a project holding a plan.
it('opens the new storyline on ③ after a request from 생성 on an existing plan', async () => {
  const job: FakeGenerationJobRow = {
    id: 'storyline-again',
    kind: 'storyline_clip',
    status: 'running',
    stage: 'storyline',
    clipProjectId: 'clip',
  }
  let started = false
  const view = mount(
    {
      readProject: (p) => ({
        ...p,
        latestJob: started ? job : undefined,
        storyline:
          job.status === 'done'
            ? {
                paragraphs: [{ text: '새로 작성한 스토리라인', observationIds: [] }],
                editedByHand: false,
                addedSourceIds: [],
                takenOutObservationIds: [],
              }
            : undefined,
      }),
    },
    [job],
  )
  await userEvent.click(await screen.findByRole('tab', { name: '생성' }))
  expect(screen.getByRole('tab', { name: '생성' })).toHaveAttribute('aria-selected', 'true')
  started = true
  await act(() =>
    view.queryClient.invalidateQueries({ queryKey: clipProjectsKey(view.transport, 'alice') }),
  )
  await screen.findByRole('progressbar', { name: '스토리라인 작성 중' })
  job.status = 'done'
  await act(() =>
    view.queryClient.invalidateQueries({ queryKey: clipProjectsKey(view.transport, 'alice') }),
  )
  await waitFor(() =>
    expect(screen.getByRole('tab', { name: '스토리라인' })).toHaveAttribute(
      'aria-selected',
      'true',
    ),
  )
  expect(await screen.findByText('새로 작성한 스토리라인')).toBeVisible()
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
  expect(screen.getByText(/이 작업은 취소 기능을 지원하지/)).toBeVisible()
})

// The workspace states ONE figure for the job — the credits it used — and for an exempt
// account the reference amount the server recorded, with no operator wording (CLIP-81,
// QUOTA-68). The ceiling, the reservation, the refund and the cancellation split stay
// the server's bookkeeping.
it.each([false, true])('states only the credits the job used (exempt=%s)', async (exempt) => {
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
            cancellationFeeCredits: 0,
            finalChargeCredits: exempt ? 0 : 4,
            refundCredits: exempt ? 0 : 17,
            ...(exempt
              ? {
                  shadowConfirmedChargeCredits: 4,
                  shadowCancellationFeeCredits: 0,
                  shadowChargeCredits: 4,
                }
              : {}),
          },
        },
      ],
    },
    [job],
  )
  await confirm()
  const credit = within(screen.getByRole('region', { name: '이번 작업의 크레딧' }))
  expect(credit.getByRole('status')).toHaveTextContent('4 크레딧 사용')
  expect(credit.getByRole('status')).not.toHaveTextContent(/마스터|차감하지/)
  if (!exempt) expect(screen.getByText('확인된 AI 사용 차감 4 크레딧')).toBeInTheDocument()
  expect(screen.queryByText('취소 추가분')).not.toBeInTheDocument()
  expect(screen.queryByText('8 크레딧')).not.toBeInTheDocument()
  expect(screen.queryByText('21 크레딧')).not.toBeInTheDocument()
})

it('opens a plan with no render as a plan to review, not as a missing result', async () => {
  // A generation now stops at the validated plan (CLIP-151), so this is ②'s
  // normal FIRST state: the draft preview, the render action, and nothing
  // anywhere saying the attempt failed or that a result went missing.
  mount({ projects: [{ ...project(), renderedPlanRevision: 0, result: undefined }] })
  expect(await screen.findByRole('tab', { name: '다듬기' })).toHaveAttribute(
    'aria-selected',
    'true',
  )
  expect(screen.getByRole('region', { name: '편집 중인 영상' })).toBeVisible()
  expect(screen.getByText('수정됨 · 다시 출력 필요')).toBeVisible()
  expect(screen.queryByRole('link', { name: /다운로드/ })).not.toBeInTheDocument()
  // ①'s retained review (hidden on ②) states the credit policy for failed work; nothing
  // visible says this attempt failed.
  for (const text of screen.queryAllByText(/실패/)) expect(text).not.toBeVisible()
  expect(screen.queryByText(/아직 다듬을 편집안이 없어요/)).not.toBeInTheDocument()
  // 확정하기 needs no render (CLIP-75): it is the step bar's big action from the first plan, and
  // ② holds no render at all — ③ renders the confirmed plan (CLIP-149, CLIP-153).
  expect(screen.getByRole('button', { name: '확정하기' })).toHaveClass('bg-button-cta-bg')
  expect(screen.queryByRole('button', { name: /^(렌더하기|다시 렌더)$/ })).not.toBeInTheDocument()
  expect(screen.queryByText(/만든 뒤 완료로 표시해 주세요/)).not.toBeInTheDocument()
})

// CLIP-36: the steps read 영상 생성 · 영상 다듬기 · 영상 완성, and a confirmed or finalized
// project opens ③ while an unconfirmed one holding a plan opens ②.
// THEME-39: the clip runs the post's five stations under the same short names.
it('names the five steps and opens a confirmed clip on ⑤', async () => {
  const clip = project()
  clip.confirmed = { planRevision: clip.editPlanRevision!, at: '2026-11-12T00:00:00Z' }
  mount({ projects: [clip] })
  expect(await screen.findByRole('tab', { name: '완성' })).toHaveAttribute('aria-selected', 'true')
  expect(screen.getAllByRole('tab').map((tab) => tab.textContent)).toEqual([
    '생성',
    '관찰',
    '스토리라인',
    '다듬기',
    '완성',
  ])
  expect(await screen.findByRole('heading', { name: '완성됐어요' })).toBeVisible()
})

it('renders the confirmed plan in ③ at once after 확정하기 on a supported device', async () => {
  const calls: string[] = []
  mount({ projects: [{ ...project(), renderedPlanRevision: 0, result: undefined }], calls })
  await userEvent.click(await confirm())
  await waitFor(() =>
    expect(screen.getByRole('tab', { name: '완성' })).toHaveAttribute('aria-selected', 'true'),
  )
  // jsdom has no encoder, so the device is unsupported: ③ offers 렌더하기 instead of starting.
  expect(await screen.findByRole('button', { name: '렌더하기' })).toBeInTheDocument()
  expect(calls).not.toContain('StartClipRender')
})

it.each(['server', 'browser'] as const)(
  'allows a matching %s render and can dismiss finalization in ③ without ending editing',
  async (renderKind) => {
    const calls: string[] = []
    const clip = project()
    clip.result!.renderKind = renderKind
    mount({ projects: [clip], calls })
    await confirm()
    await selectCut()
    await userEvent.click(await confirm())
    await userEvent.click(await screen.findByRole('button', { name: '완료로 표시' }))
    const dialog = within(await screen.findByRole('dialog', { name: '클립을 완료로 표시할까요?' }))
    expect(dialog.getByText(/완료로 표시하면 원본을 삭제하고 수정이 끝나요/)).toBeVisible()
    expect(calls).not.toContain('FinalizeClipProject')
    await userEvent.click(dialog.getByRole('button', { name: '취소' }))
    expect(
      screen.queryByRole('dialog', { name: '클립을 완료로 표시할까요?' }),
    ).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('tab', { name: '다듬기' }))
    await selectCut()
    expect(screen.getByRole('button', { name: '컷 삭제' })).toBeEnabled()
    expect(calls).not.toContain('FinalizeClipProject')
  },
)
