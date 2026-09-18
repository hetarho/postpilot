import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { renderAppAt } from '@/test/app'
import { observedClipFixture } from '@/test/clip-observations'

describe('observations in the clip workspace', () => {
  it('mounts only the chosen reference tab and keeps the generation step references in flow', async () => {
    const project = observedClipFixture()
    project.requests = [
      { kind: 'instruction', body: '원문 요청을 그대로 보관', createdAt: '2026-09-18T00:00:00Z' },
    ]
    renderAppAt('/clips/project', { user: { id: 'alice' }, clips: { projects: [project] } })
    const open = await screen.findByRole('button', { name: '참고 자료' })
    expect(screen.queryByRole('heading', { name: 'AI가 관찰한 내용' })).not.toBeInTheDocument()
    expect(screen.queryByLabelText('원본 영상 선택')).not.toBeInTheDocument()
    expect(screen.queryByText('원문 요청을 그대로 보관')).not.toBeInTheDocument()
    await userEvent.click(open)
    const sheet = within(screen.getByRole('dialog', { name: '참고 자료' }))
    expect(sheet.getByRole('tab', { name: '관찰 기록' })).toHaveAttribute('aria-selected', 'true')
    expect(sheet.getByRole('heading', { name: 'AI가 관찰한 내용' })).toBeVisible()
    await userEvent.click(sheet.getByRole('button', { name: 'source-b.mp4 선택' }))
    expect(sheet.getByRole('button', { name: 'source-b.mp4 선택' })).toHaveAttribute(
      'aria-pressed',
      'true',
    )
    await userEvent.click(sheet.getByRole('tab', { name: '원본 영상' }))
    expect(screen.queryByRole('heading', { name: 'AI가 관찰한 내용' })).not.toBeInTheDocument()
    expect(sheet.getByLabelText('원본 영상 선택')).toBeInTheDocument()
    expect(sheet.getByRole('heading', { name: '다시 출력에 필요한 원본' })).toBeVisible()
    await userEvent.click(sheet.getByRole('tab', { name: '요청 기록' }))
    expect(screen.queryByLabelText('원본 영상 선택')).not.toBeInTheDocument()
    await userEvent.click(sheet.getByText('AI에 요청한 내용 1건'))
    expect(sheet.getByText('원문 요청을 그대로 보관')).toBeVisible()
    await userEvent.keyboard('{Escape}')
    expect(screen.queryByText('원문 요청을 그대로 보관')).not.toBeInTheDocument()
    await userEvent.click(open)
    expect(screen.getByRole('tab', { name: '관찰 기록' })).toHaveAttribute('aria-selected', 'true')
    await userEvent.keyboard('{Escape}')
    await userEvent.click(screen.getByRole('tab', { name: '클립 생성' }))
    expect(screen.queryByRole('button', { name: '참고 자료' })).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'AI가 관찰한 내용' })).toBeVisible()
    expect(screen.getByLabelText('원본 영상 선택')).toBeInTheDocument()
    expect(screen.getByText('원문 요청을 그대로 보관')).toBeInTheDocument()
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
    await userEvent.click(tabs.getByRole('tab', { name: /클립 생성/ }))
    expect(screen.getByRole('heading', { name: '이번 작업에서 확인할 수 있는 내용' })).toBeVisible()
    expect(calls).not.toContain('StartClipGeneration')
    expect(calls).not.toContain('StartClipRender')
    expect(calls).not.toContain('QuoteClipGeneration')
  })
  it('keeps observations available on both editable steps without invoking generation', async () => {
    const calls: string[] = []
    const project = observedClipFixture()
    renderAppAt('/clips/project', { user: { id: 'alice' }, clips: { projects: [project], calls } })
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: '참고 자료' }))
    expect(await screen.findByRole('heading', { name: 'AI가 관찰한 내용' })).toBeVisible()
    await user.keyboard('{Escape}')
    const tabs = within(screen.getByRole('tablist', { name: '클립 단계' }))
    for (const name of ['클립 생성', '클립 다듬기']) {
      await user.click(tabs.getByRole('tab', { name: new RegExp(name) }))
      if (name === '클립 다듬기')
        await user.click(screen.getByRole('button', { name: '참고 자료' }))
      expect(screen.getAllByRole('heading', { name: 'AI가 관찰한 내용' })).toHaveLength(1)
      await user.click(screen.getByRole('button', { name: '관찰 구간 2개 자세히 보기' }))
      expect(screen.getByText('1번 컷에 사용 · 1× · 원본 0:03.500–0:10')).toBeVisible()
    }
    expect(calls).not.toContain('StartClipGeneration')
    expect(calls).not.toContain('QuoteClipGeneration')
    expect(calls).not.toContain('CreateClipSourceBatch')
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
      await userEvent.click(await screen.findByRole('button', { name: '참고 자료' }))
      expect(await screen.findByText(/이전 생성의 관찰 결과예요/)).toBeVisible()
      await userEvent.click(screen.getByRole('button', { name: /자세히 보기/ }))
      expect(screen.getByText('맛있어요')).toBeVisible()
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
