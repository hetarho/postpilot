import { discardClipObservationQueues } from '@/features/inspect-clip-observations'
import { Code } from '@connectrpc/connect'
import { connectAppError } from '@/test/app-error'
import { clipGuideTo } from '@/test/clip-preparation'
import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
import { renderAppAt } from '@/test/app'
import { observedClipFixture } from '@/test/clip-observations'

describe('observations in the clip workspace', () => {
  // CLIP-150, CLIP-48: ④'s reference holds 원본 영상 and 요청 기록, ② shows the observations and
  // ① keeps the source strip and the request record in flow.
  it('mounts only the chosen reference tab, shows observations on ② and keeps ①’s references in flow', async () => {
    const project = observedClipFixture()
    project.requests = [
      { kind: 'instruction', body: '원문 요청을 그대로 보관', createdAt: '2026-09-18T00:00:00Z' },
    ]
    renderAppAt('/clips/project', { user: { id: 'alice' }, clips: { projects: [project] } })
    const open = await screen.findByRole('button', { name: '원본 소스' })
    expect(screen.queryByRole('heading', { name: 'AI가 관찰한 내용' })).not.toBeInTheDocument()
    expect(
      within(screen.getByRole('region', { name: '컷·자막 수정' })).queryByLabelText(
        '원본 영상 선택',
      ),
    ).not.toBeInTheDocument()
    expect(
      within(screen.getByRole('region', { name: '컷·자막 수정' })).queryByText(
        '원문 요청을 그대로 보관',
      ),
    ).not.toBeInTheDocument()
    await userEvent.click(open)
    const sheet = within(screen.getByRole('dialog', { name: '원본 소스' }))
    expect(sheet.queryByRole('tab', { name: '관찰 기록' })).not.toBeInTheDocument()
    expect(sheet.getByRole('tab', { name: '원본 영상' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.queryByRole('heading', { name: 'AI가 관찰한 내용' })).not.toBeInTheDocument()
    expect(sheet.getByLabelText('원본 영상 선택')).toBeInTheDocument()
    expect(sheet.getByRole('heading', { name: '다시 출력에 필요한 원본' })).toBeVisible()
    await userEvent.click(sheet.getByRole('tab', { name: '요청 기록' }))
    expect(sheet.queryByLabelText('원본 영상 선택')).not.toBeInTheDocument()
    await userEvent.click(sheet.getByText('AI에 요청한 내용 1건'))
    expect(sheet.getByText('원문 요청을 그대로 보관')).toBeVisible()
    await userEvent.keyboard('{Escape}')
    expect(
      within(screen.getByRole('region', { name: '컷·자막 수정' })).queryByText(
        '원문 요청을 그대로 보관',
      ),
    ).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('tab', { name: '관찰' }))
    expect(screen.getByRole('heading', { name: 'AI가 관찰한 내용' })).toBeVisible()
    await userEvent.click(screen.getByRole('button', { name: 'source-b.mp4 선택' }))
    expect(screen.getByRole('button', { name: 'source-b.mp4 선택' })).toHaveAttribute(
      'aria-pressed',
      'true',
    )
    await userEvent.click(screen.getByRole('tab', { name: '생성' }))
    expect(screen.queryByRole('button', { name: '원본 소스' })).not.toBeInTheDocument()
    await clipGuideTo('sources')
    expect(screen.queryByRole('heading', { name: 'AI가 관찰한 내용' })).not.toBeInTheDocument()
    expect(screen.getByLabelText('원본 영상 선택')).toBeInTheDocument()
    expect(
      within(screen.getByRole('region', { name: '어떤 영상으로 만들까요?' })).getByText(
        '원문 요청을 그대로 보관',
      ),
    ).toBeInTheDocument()
  })

  it('reopens failed attempt work on both editable steps without replacing the previous video', async () => {
    const project = observedClipFixture()
    const job = { id: 'failed-attempt', kind: 'generate_clip', status: 'failed', stage: 'plan' }
    const calls: string[] = []
    renderAppAt('/clips/project', {
      user: { id: 'alice' },
      clips: {
        calls,
        projects: [
          {
            ...project,
            latestJob: job,
            attemptInspection: {
              jobId: job.id,
              status: 'available',
              stage: 'plan',
              completedChunks: 2,
              totalChunks: 2,
              completedSources: 2,
              totalSources: 2,
              observations: { status: 'empty', sources: [] },
              ranges: [],
              validationCheck: 'plan_timeline',
              validationPhase: 'timeline_grow',
              measurements: { after_ms: 12000 },
            },
          },
        ],
      },
      jobs: { jobs: [job] },
    })
    expect(
      await screen.findByRole('heading', { name: '이번 작업에서 확인할 수 있는 내용' }),
    ).toBeVisible()
    expect(screen.getByText('원본 분석 2 / 2 구간 완료')).toBeVisible()
    const tabs = within(screen.getByRole('tablist', { name: '클립 단계' }))
    await userEvent.click(tabs.getByRole('tab', { name: '생성' }))
    expect(screen.getByRole('heading', { name: '이번 작업에서 확인할 수 있는 내용' })).toBeVisible()
    expect(calls).not.toContain('StartClipGeneration')
    expect(calls).not.toContain('StartClipRender')
    expect(calls).not.toContain('QuoteClipGeneration')
  })
  it('shows the observations with their usage on ② without invoking generation', async () => {
    const calls: string[] = []
    const project = observedClipFixture()
    renderAppAt('/clips/project', { user: { id: 'alice' }, clips: { projects: [project], calls } })
    const user = userEvent.setup()
    const tabs = within(await screen.findByRole('tablist', { name: '클립 단계' }))
    await user.click(tabs.getByRole('tab', { name: '관찰' }))
    expect(screen.getAllByRole('heading', { name: 'AI가 관찰한 내용' })).toHaveLength(1)
    await user.click(screen.getByRole('button', { name: '관찰 구간 2개 자세히 보기' }))
    expect(
      within(screen.getByRole('tabpanel', { name: '관찰' })).getByText(
        '1번 컷에 사용 · 1× · 원본 0:03.500–0:10',
      ),
    ).toBeVisible()
    expect(calls).not.toContain('StartClipGeneration')
    expect(calls).not.toContain('QuoteClipGeneration')
    expect(calls).not.toContain('CreateClipSourceBatch')
  })

  // CLIP-235: ② corrects an observed scene's descriptive text in place and autosaves it; the
  // saved correction comes back marked as the owner's, and 다시 관찰하기 stands in the heading row.
  it('autosaves an in-place correction on ② and marks it as the owner’s', async () => {
    const calls: string[] = [],
      edits: unknown[] = []
    const project = observedClipFixture()
    renderAppAt('/clips/project', {
      user: { id: 'alice' },
      clips: { projects: [project], calls, observationEdits: edits },
    })
    const user = userEvent.setup()
    const tabs = within(await screen.findByRole('tablist', { name: '클립 단계' }))
    await user.click(tabs.getByRole('tab', { name: '관찰' }))
    const panel = within(screen.getByRole('tabpanel', { name: '관찰' }))
    expect(panel.getByRole('button', { name: '다시 관찰하기' })).toBeInTheDocument()
    await user.click(panel.getByRole('button', { name: '관찰 구간 2개 자세히 보기' }))
    const event = panel.getAllByRole('textbox', { name: '장면' })[0]!
    await user.clear(event)
    await user.type(event, '국수를 집어 든다')
    await waitFor(() => expect(edits).toHaveLength(1), { timeout: 3000 })
    expect(edits[0]).toMatchObject({
      clipId: 'project',
      edits: [{ observationId: 'a/0', event: '국수를 집어 든다' }],
    })
    expect(await panel.findAllByText('직접 고침')).not.toHaveLength(0)
    // CLIP-49's usage stays beside the corrected text.
    expect(panel.getByText('1번 컷에 사용 · 1× · 원본 0:03.500–0:10')).toBeVisible()
    expect(calls).not.toContain('StartClipAnalysis')
  })

  it.each(['failed', 'cancelled'])(
    'identifies previous observations during a %s regeneration',
    async (status) => {
      renderAppAt('/clips/project', {
        user: { id: 'alice' },
        clips: {
          projects: [
            {
              ...observedClipFixture(),
              latestJob: { id: 'new-run', kind: 'generate_clip', status, stage: 'analyze' },
            },
          ],
        },
        jobs: { jobs: [{ id: 'new-run', kind: 'generate_clip', status, stage: 'analyze' }] },
      })
      await userEvent.click(await screen.findByRole('tab', { name: '관찰' }))
      const reference = within(screen.getByRole('tabpanel', { name: '관찰' }))
      expect(await reference.findByText(/이전 생성의 관찰 결과예요/)).toBeVisible()
      await userEvent.click(screen.getByRole('button', { name: /자세히 보기/ }))
      expect(reference.getByText('맛있어요')).toBeVisible()
    },
  )
})

it.each(['queued', 'running'])('hides observations during %s work', async (status) => {
  const job = { id: 'new-run', kind: 'generate_clip', status, stage: 'analyze' }
  renderAppAt('/clips/project', {
    user: { id: 'alice' },
    clips: { projects: [{ ...observedClipFixture(), latestJob: job }] },
    jobs: { jobs: [job] },
  })
  await screen.findByRole('progressbar')
  expect(screen.queryByRole('heading', { name: 'AI가 관찰한 내용' })).not.toBeInTheDocument()
  expect(screen.queryByRole('tablist', { name: '클립 단계' })).not.toBeInTheDocument()
})

it('keeps a failed observation save on the shared status line after changing steps', async () => {
  const user = userEvent.setup()
  renderAppAt('/clips/project', {
    user: { id: 'alice' },
    clips: {
      projects: [observedClipFixture()],
      observationSaveGate: async () => {
        throw connectAppError('CLIP_INVALID_INPUT', Code.InvalidArgument)
      },
    },
  })
  await user.click(await screen.findByRole('tab', { name: '관찰' }))
  await user.click(screen.getByRole('button', { name: '관찰 구간 2개 자세히 보기' }))
  fireEvent.change(screen.getAllByRole('textbox', { name: '장면' })[0]!, {
    target: { value: 'owner correction' },
  })
  const line = await screen.findByRole('status', { name: '클립 상태' })
  await waitFor(() => expect(line).toHaveTextContent('고친 관찰 내용을 저장하지 못했어요.'))
  await user.click(screen.getByRole('tab', { name: '스토리라인' }))
  expect(line).toBeVisible()
  expect(line).toHaveTextContent('고친 관찰 내용을 저장하지 못했어요.')
  expect(screen.getAllByText(/고친 관찰 내용을 저장하지 못했어요/)).toHaveLength(1)
})

afterEach(() => discardClipObservationQueues())
