import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { observedClipFixture, clipObservationsFixture } from '@/test/clip-observations'
import { ClipAttemptInspection } from './ClipAttemptInspection'

afterEach(() => {
  initializeI18n('ko')
  vi.restoreAllMocks()
})

function failedProject() {
  const p = observedClipFixture()
  p.latestJob = {
    id: 'attempt',
    kind: 'generate_clip',
    status: 'failed',
    stage: 'plan',
    progressDone: 0,
    progressTotal: 1,
    failure: undefined,
    postSlug: '',
    observeModel: undefined,
    writeModel: undefined,
    createdAt: '2026-09-13T09:00:00Z',
    updatedAt: '2026-09-13T09:01:00Z',
    targetLanguage: undefined,
  }
  const observations = clipObservationsFixture()
  observations.sources[0]!.segments[0]!.event = '이번 시도의 관찰'
  p.attemptInspection = {
    jobId: 'attempt',
    status: 'available',
    stage: 'plan',
    completedChunks: 2,
    totalChunks: 3,
    completedSources: 1,
    totalSources: 2,
    observations,
    ranges: [
      { cut: 1, source: 1, startMs: 3500, endMs: 10500, valid: true },
      { cut: 2, source: 0, startMs: 0, endMs: 0, valid: false },
    ],
    validationCheck: 'plan_timeline',
    validationPhase: 'timeline_grow',
    measurements: { target_ms: 30000, before_ms: 6000, after_ms: 12000, remaining_ms: 18000 },
  }
  return p
}

it('shows only this attempt evidence, plays original ranges and preserves the previous result', async () => {
  const p = failedProject()
  const before = structuredClone(p.result)
  const resolve = vi.fn().mockResolvedValue('https://original.test/video')
  const pause = vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
  render(<ClipAttemptInspection project={p} localSources={[]} resolvePlayback={resolve} />)
  expect(screen.getByText('원본 분석 2 / 3 구간 완료')).toBeVisible()
  expect(screen.getAllByText('이번 시도의 관찰')[0]).toBeVisible()
  expect(screen.queryByText('접시에 담긴 음식을 가까이 촬영')).not.toBeInTheDocument()
  expect(screen.queryByText(/사용 구간은 완성 영상/)).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: /컷 2/ })).toBeDisabled()
  await userEvent.click(screen.getByRole('button', { name: '컷 1 보기' }))
  const video = (await screen.findByLabelText('선택된 원본 구간 재생')) as HTMLVideoElement
  fireEvent.loadedMetadata(video)
  expect(video.currentTime).toBe(3.5)
  video.currentTime = 10.5
  fireEvent.timeUpdate(video)
  expect(pause).toHaveBeenCalled()
  fireEvent.play(video)
  expect(video.currentTime).toBe(3.5)
  expect(p.result).toEqual(before)
  expect(screen.queryByRole('button', { name: /다운로드|확정/ })).not.toBeInTheDocument()
})

it('shows legacy absence, hides superseded work and never borrows successful observations', () => {
  const p = failedProject()
  p.attemptInspection = undefined
  const { rerender } = render(<ClipAttemptInspection project={p} localSources={[]} />)
  expect(screen.getByText(/이 작업에는 중간 기록이 없어요/)).toBeVisible()
  expect(screen.queryByText('접시에 담긴 음식을 가까이 촬영')).not.toBeInTheDocument()
  rerender(
    <ClipAttemptInspection
      project={failedProject()}
      currentJobId="new-attempt"
      localSources={[]}
    />,
  )
  expect(screen.queryByRole('heading')).not.toBeInTheDocument()
})

it('keeps completed observations visible when original playback fails and when reopened in English', async () => {
  const p = failedProject()
  const resolve = vi.fn().mockRejectedValue(new Error('expired'))
  const { unmount } = render(
    <ClipAttemptInspection project={p} localSources={[]} resolvePlayback={resolve} />,
  )
  await userEvent.click(screen.getByRole('button', { name: '컷 1 보기' }))
  expect(await screen.findByText(/이 원본을 재생할 수 없어요/)).toBeVisible()
  expect(screen.getAllByText('이번 시도의 관찰')[0]).toBeVisible()
  unmount()
  initializeI18n('en')
  render(<ClipAttemptInspection project={p} localSources={[]} />)
  expect(screen.getByRole('heading', { name: 'Work available from this attempt' })).toBeVisible()
  expect(screen.getByText('Original analysis: 2 of 3 chunks completed')).toBeVisible()
  expect(screen.getAllByText('이번 시도의 관찰')[0]).toBeVisible()
})

it.each(['ko', 'en'] as const)(
  'explains observation geometry and preserves historical uncertainty in %s',
  (language) => {
    initializeI18n(language)
    const p = failedProject()
    p.latestJob!.stage = 'analyze'
    Object.assign(p.attemptInspection!, {
      stage: 'analyze',
      completedChunks: 19,
      totalChunks: 20,
      validationPhase: 'observation',
      validationCheck: 'observe_subject_bounds',
      measurements: {
        source: 20,
        chunk: 20,
        segment: 1,
        duration_ms: 3115,
        subject_x_ppm: -100000,
        subject_width_ppm: 1200000,
      },
    })
    const { rerender } = render(<ClipAttemptInspection project={p} localSources={[]} />)
    expect(
      screen.getByText(
        language === 'ko'
          ? 'AI가 표시한 초점이나 등장 대상의 위치·크기가 올바르지 않았어요.'
          : 'The focal point or the subject’s position or size was invalid.',
      ),
    ).toBeVisible()
    expect(screen.getByText('-10%')).toBeVisible()
    expect(screen.getByText('120%')).toBeVisible()
    expect(
      screen.getByText(language === 'ko' ? '확인이 필요한 관찰 구간' : 'Observation segment'),
    ).toBeVisible()
    expect(screen.getAllByText('이번 시도의 관찰')[0]).toBeVisible()
    p.attemptInspection!.validationCheck = 'unknown'
    rerender(<ClipAttemptInspection project={p} localSources={[]} />)
    expect(
      screen.getByText(
        language === 'ko' ? /세부 검증 사유가 기록되지 않은/ : /no detailed validation reason/,
      ),
    ).toBeVisible()
  },
)
