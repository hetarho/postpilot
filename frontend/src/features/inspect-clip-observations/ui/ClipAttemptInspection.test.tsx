/// <reference types="node" />
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
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

it.each(['ko', 'en'] as const)(
  'keeps completed observations beside an actionable media failure in %s',
  (language) => {
    initializeI18n(language)
    const project = failedProject()
    project.latestJob!.failure = { reason: 'CLIP_MEDIA_RETRY_EXHAUSTED', params: {} }
    project.latestJob!.stage = 'render_retry'
    project.attemptInspection!.stage = 'render_retry'
    project.attemptInspection!.validationCheck = 'unknown'
    render(<ClipAttemptInspection project={project} localSources={[]} />)
    expect(
      screen.getByText(language === 'ko' ? /잠시 후 다시 시도하세요/ : /Try again later/),
    ).toBeVisible()
    expect(screen.getAllByText('이번 시도의 관찰')[0]).toBeVisible()
    expect(
      screen.getByText(
        language === 'ko'
          ? '원본 분석 2 / 3 구간 완료'
          : 'Original analysis: 2 of 3 chunks completed',
      ),
    ).toBeVisible()
  },
)

it.each(['ko', 'en'] as const)(
  'shows the authoritative historical element error in %s',
  (language) => {
    initializeI18n(language)
    const project = failedProject()
    project.attemptInspection!.validationCheck = 'unknown'
    project.latestJob!.failure = {
      reason: 'CLIP_COMPOSITION_INVALID',
      params: { element_id: 'disclosure_badge', line: '39', reason: 'invalid_style' },
    }
    render(<ClipAttemptInspection project={project} localSources={[]} />)
    expect(screen.getByText(/disclosure_badge/)).toBeVisible()
    expect(
      screen.queryByText(
        language === 'ko' ? /세부 검증 사유가 기록되지 않은/ : /no detailed validation reason/,
      ),
    ).not.toBeInTheDocument()
  },
)

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

it.each(['ko', 'en'] as const)(
  'explains input overflow with the actual limit in %s',
  (language) => {
    initializeI18n(language)
    const p = failedProject()
    Object.assign(p.attemptInspection!, {
      validationCheck: 'input_prompt_limit',
      validationPhase: 'input',
      measurements: { input_bytes: 37198, input_limit_bytes: 27952 },
      ranges: [],
    })
    render(<ClipAttemptInspection project={p} localSources={[]} />)
    expect(screen.getByText(language === 'ko' ? '37198바이트' : '37198 bytes')).toBeVisible()
    expect(screen.getByText(language === 'ko' ? '27952바이트' : '27952 bytes')).toBeVisible()
    expect(
      screen.getByText(
        language === 'ko'
          ? /템플릿을 줄이거나 원본 수를/
          : /Shorten the template or select fewer sources/,
      ),
    ).toBeVisible()
    expect(
      screen.queryByText(
        language === 'ko' ? /세부 검증 사유가 기록되지/ : /no detailed validation reason/,
      ),
    ).not.toBeInTheDocument()
    expect(screen.getAllByText('이번 시도의 관찰')[0]).toBeVisible()
  },
)

it.each(['ko', 'en'] as const)(
  'names the delivered property a refused render missed in %s',
  (language) => {
    initializeI18n(language)
    const project = failedProject()
    project.latestJob!.stage = 'render'
    project.latestJob!.failure = { reason: 'CLIP_PROCESSING_FAILED', params: {} }
    project.attemptInspection!.stage = 'render'
    project.attemptInspection!.validationCheck = 'render_output_duration'
    project.attemptInspection!.validationPhase = 'render'
    project.attemptInspection!.measurements = {
      duration_ms: 24000,
      expected_duration_ms: 25334,
      decoded_duration_ms: 24000,
      container_duration_ms: 24000,
    }
    render(<ClipAttemptInspection project={project} localSources={[]} />)
    // The generic reason stays: it is what tells the owner what to do next.
    expect(
      screen.getByText(language === 'ko' ? /클립 처리에 실패했어요/ : /Clip processing failed/),
    ).toBeVisible()
    expect(
      screen.getByText(
        language === 'ko' ? /완성된 영상의 길이가 편집안의 길이와/ : /more than one frame away/,
      ),
    ).toBeVisible()
    expect(screen.getByText(language === 'ko' ? '편집안의 길이' : 'Planned duration')).toBeVisible()
  },
)

// Exercise the public diagnostic vocabulary itself, so a newly projected backend
// check cannot silently fall back to a generic failure again.
const safeChecks = [
  ...readFileSync(
    resolve(import.meta.dirname, '../../../../../backend/internal/clip/attempt_diagnostics.go'),
    'utf8',
  )
    .split('func SafeAttemptCheck(check string) string {')[1]!
    .matchAll(/"([a-z_]+)"/g),
]
  .map((match) => match[1]!)
  .filter((check) => check !== 'unknown')

it.each(['ko', 'en'] as const)(
  'shows a localized checkpoint beside a structured failure for every public check in %s',
  (language) => {
    initializeI18n(language)
    expect(safeChecks.length).toBeGreaterThan(80)
    const project = failedProject()
    project.latestJob!.failure = { reason: 'CLIP_PROCESSING_FAILED', params: {} }
    project.attemptInspection!.validationPhase = 'validation'
    project.attemptInspection!.observations.sources = []
    const { rerender } = render(<ClipAttemptInspection project={project} localSources={[]} />)
    for (const check of safeChecks) {
      project.attemptInspection!.validationCheck = check
      rerender(<ClipAttemptInspection project={project} localSources={[]} />)
      const reason = screen.getByText(
        language === 'ko' ? /클립 처리에 실패했어요/ : /Clip processing failed/,
      )
      const explanation = reason.nextElementSibling
      expect(explanation?.tagName, check).toBe('P')
      expect(explanation, check).toHaveClass('text-content-secondary')
      expect(explanation?.textContent, check).not.toMatch(/inspection\.|validationFailed/)
      expect(explanation?.textContent?.length, check).toBeGreaterThan(10)
      expect(explanation, check).toBeVisible()
    }
  },
)

it.each(
  (['ko', 'en'] as const).flatMap((language) =>
    (['CLIP_PROCESSING_FAILED', 'MODEL_OUTPUT_INVALID'] as const).map((reason) => ({
      language,
      reason,
    })),
  ),
)('keeps $reason and the template section order together in $language', ({ language, reason }) => {
  initializeI18n(language)
  const project = failedProject()
  project.latestJob!.failure = { reason, params: {} }
  project.attemptInspection!.validationCheck = 'composition_section_order'
  project.attemptInspection!.validationPhase = 'selection'
  project.attemptInspection!.measurements = { cut: 2 }
  render(<ClipAttemptInspection project={project} localSources={[]} />)
  expect(
    screen.getByText(
      reason === 'CLIP_PROCESSING_FAILED'
        ? language === 'ko'
          ? /클립 처리에 실패했어요/
          : /Clip processing failed/
        : language === 'ko'
          ? 'AI 결과 형식을 읽을 수 없어요.'
          : 'The AI response format could not be read.',
    ),
  ).toBeVisible()
  expect(
    screen.getByText(
      language === 'ko'
        ? '편집안의 섹션이나 항목 순서가 템플릿에 정해진 순서와 맞지 않았어요.'
        : 'The plan did not follow the template’s section or item order.',
    ),
  ).toBeVisible()
  expect(screen.getByText('2')).toBeVisible()
})

it.each(['unknown', 'future_check'])(
  'preserves structured failure wording for an unrecognized check: %s',
  (check) => {
    const project = failedProject()
    project.latestJob!.failure = { reason: 'MODEL_OUTPUT_INVALID', params: {} }
    project.attemptInspection!.validationCheck = check
    render(<ClipAttemptInspection project={project} localSources={[]} />)
    expect(screen.getByText('AI 결과 형식을 읽을 수 없어요.')).toBeVisible()
    expect(screen.queryByText(/세부 검증 사유가 기록되지 않은/)).not.toBeInTheDocument()
    expect(screen.queryByText(/선택한 구간을 늘려도/)).not.toBeInTheDocument()
  },
)

it.each([
  ['timeline_grow', /15초/],
  ['timeline_shrink', /선택한 구간을 줄여도 목표 길이/],
  ['timeline_total', /전환/],
] as const)('keeps phase-specific timeline guidance for %s', (phase, message) => {
  const project = failedProject()
  project.attemptInspection!.validationPhase = phase
  render(<ClipAttemptInspection project={project} localSources={[]} />)
  expect(screen.getByText(message)).toBeVisible()
})
