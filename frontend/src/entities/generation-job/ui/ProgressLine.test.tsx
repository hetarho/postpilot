import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import type { GenerationJob } from '../model/types'
import { FailureNotice } from './FailureNotice'
import { ProgressLine } from './ProgressLine'

const JOB: GenerationJob = {
  id: 'job-1',
  kind: 'generate',
  status: 'running',
  stage: 'observe',
  progressDone: 3,
  progressTotal: 8,
  failure: undefined,
  postSlug: 'post-a',
  observeModel: undefined,
  writeModel: undefined,
  createdAt: '',
  updatedAt: '',
  targetLanguage: 'ko',
}

it('names the running stage as a live status, with no counts in the prose', () => {
  render(<ProgressLine job={JOB} />)
  expect(screen.getByRole('status')).toHaveTextContent('사진 관찰 중')
  expect(screen.getByRole('status')).not.toHaveTextContent('3/8')
})

// The voice screen's seed run reports a stage nothing here knows, and it used to render the
// literal sentence 작업 준비 중 for the whole run.
it('reports an unrecognized stage as a running job rather than as a job not yet started', () => {
  render(<ProgressLine job={{ ...JOB, stage: 'seed' }} />)
  expect(screen.getByRole('status')).toHaveTextContent('생성 중')
})

it('shows the stable failure and delegates retry to its caller', async () => {
  const retry = vi.fn()
  render(<FailureNotice failure={{ reason: 'MODEL_RATE_LIMITED', params: {} }} onRetry={retry} />)
  expect(screen.getByRole('alert')).toHaveTextContent('AI 모델 요청이 많아')
  await userEvent.click(screen.getByRole('button', { name: '다시 시도' }))
  expect(retry).toHaveBeenCalledOnce()
})
