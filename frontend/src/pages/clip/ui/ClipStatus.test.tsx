import { expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import type { GenerationJob } from '@/entities/generation-job'
import type { ClipProject } from '@/entities/clip-project'
import type { ClipUploadState } from '@/features/upload-clip-sources'
import {
  ClipProgressBar,
  ClipStatusLine,
  type CorrectionStatus,
  type SaveStatus,
} from './ClipStatus'

const job = (over: Partial<GenerationJob> = {}): GenerationJob => ({
  id: 'job',
  kind: 'generate_clip',
  status: 'running',
  stage: 'analyze',
  progressDone: 1,
  progressTotal: 3,
  failure: undefined,
  postSlug: '',
  clipProjectId: 'clip',
  observeModel: undefined,
  writeModel: undefined,
  createdAt: '2026-09-11',
  updatedAt: '2026-09-11',
  targetLanguage: undefined,
  ...over,
})
const result = {
  contentType: 'video/mp4',
  bytes: 5,
  durationMs: 15000,
  createdAt: '2026-09-10T00:00:00Z',
}
const project = (over: Partial<ClipProject> = {}): ClipProject => ({
  id: 'clip',
  title: '제주',
  videoTemplateId: 'template',
  ratio: 'vertical',
  targetDurationMs: 15000,
  answers: [],
  disclosure: 'ad',
  cta: '',
  createdAt: '2026-09-11',
  updatedAt: '2026-09-11',
  editPlanRevision: 0,
  renderedPlanRevision: 0,
  ...over,
})
const QUIET: SaveStatus = { failing: false, label: '' }

function line(
  over: {
    project?: ClipProject
    job?: GenerationJob
    phase?: ClipUploadState['phase']
    correction?: CorrectionStatus
    save?: SaveStatus
  } = {},
) {
  render(
    <ClipStatusLine
      project={over.project ?? project()}
      job={over.job}
      upload={{ phase: over.phase ?? 'idle' }}
      correction={over.correction ?? 'clean'}
      save={over.save ?? QUIET}
      className="order-last w-full"
    />,
  )
  return screen.getByRole('status', { name: '클립 상태' })
}

// CLIP-38's precedence, one level at a time: each case supplies everything BELOW it too, so the
// assertion is that the higher thing wins rather than that it renders at all.
it('says a failing save before a running job, and marks it', () => {
  const status = line({
    save: { failing: true, label: '저장하지 못했어요' },
    job: job(),
    phase: 'uploading',
    correction: 'dirty',
  })
  expect(status).toHaveTextContent('저장하지 못했어요')
  expect(status.className).toContain('text-notice-danger-fg')
})

it("says a running job's stage before the upload phase", () => {
  expect(line({ job: job(), phase: 'uploading', correction: 'dirty' })).toHaveTextContent(
    '영상 분석',
  )
})

it('says the upload phase before an unsaved correction', () => {
  expect(line({ phase: 'uploading', correction: 'dirty' })).toHaveTextContent(
    '원본 영상을 올리고 확인하는 중이에요',
  )
})

// At rest the line says nothing and takes no row (owner decision 2026-09-19): the project's own
// state is the step bar's to show, and a finished upload is not an event.
it('stays quiet about an idle picker and a project at rest, out of the flow', () => {
  const status = line({
    project: project({ editPlanRevision: 2, renderedPlanRevision: 2, result }),
  })
  expect(status).toHaveTextContent('')
  expect(status).toHaveClass('sr-only')
  expect(status).not.toHaveClass('w-full')
})

it('stays quiet about an upload that is ready and waiting', () => {
  expect(line({ phase: 'ready' })).toHaveTextContent('')
})

it('takes the row only while it has something to say', () => {
  const status = line({ correction: 'unrendered' })
  expect(status).toHaveClass('w-full', 'order-last')
  expect(status).not.toHaveClass('sr-only')
})

it('says an unsaved correction before the save state', () => {
  expect(
    line({ correction: 'dirty', save: { failing: false, label: '저장했어요' } }),
  ).toHaveTextContent('저장하지 않은 수정사항이 있어요')
})

it('says a saved correction still owes a render', () => {
  expect(line({ correction: 'unrendered' })).toHaveTextContent('수정됨 · 다시 출력 필요')
})

it('says the save state before the project state', () => {
  expect(line({ save: { failing: false, label: '저장했어요' } })).toHaveTextContent('저장했어요')
})

it('says nothing for a fresh draft', () => {
  expect(line()).toHaveTextContent('')
})

it('says nothing for a project whose plan is newer than its render, beyond the correction state', () => {
  expect(
    line({ project: project({ editPlanRevision: 3, renderedPlanRevision: 2, result }) }),
  ).toHaveTextContent('')
})

it('names the stage a failed attempt stopped at', () => {
  expect(line({ job: job({ status: 'failed', stage: 'prepare' }) })).toHaveTextContent(
    '원본 준비 중 단계에서 실패했어요',
  )
})

it('is one mounted live region whose text changes rather than a swapped node', () => {
  const status = line({ save: { failing: false, label: '저장했어요' } })
  expect(status).toHaveAttribute('aria-live', 'polite')
  expect(status.tagName).toBe('DIV')
})

it('tracks a running stage and then the bytes actually put', () => {
  const { rerender } = render(
    <ClipProgressBar job={job()} upload={{ phase: 'idle', entries: [] }} />,
  )
  expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '1')
  expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuemax', '3')

  const entries = [
    { metadata: { bytes: 100 }, percent: 50 },
    { metadata: { bytes: 100 }, percent: 0 },
  ] as unknown as ClipUploadState['entries']
  rerender(<ClipProgressBar job={undefined} upload={{ phase: 'uploading', entries }} />)
  expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '5000')
  expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuemax', '20000')
})

it('paints no bar when nothing is running or uploading', () => {
  render(
    <ClipProgressBar
      job={job({ status: 'done', stage: 'cleanup' })}
      upload={{ phase: 'owned', entries: [] }}
    />,
  )
  expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
})
