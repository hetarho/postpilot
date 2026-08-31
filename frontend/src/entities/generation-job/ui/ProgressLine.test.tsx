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

it('renders the stage-specific progress copy as a live status', () => {
  render(<ProgressLine job={JOB} />)
  expect(screen.getByRole('status')).toHaveTextContent('사진 3/8 관찰됨')
})

it('shows the stable failure and delegates retry to its caller', async () => {
  const retry = vi.fn()
  render(<FailureNotice failure={{ reason: 'MODEL_RATE_LIMITED', params: {} }} onRetry={retry} />)
  expect(screen.getByRole('alert')).toHaveTextContent('AI 모델 요청이 많아')
  await userEvent.click(screen.getByRole('button', { name: '다시 시도' }))
  expect(retry).toHaveBeenCalledOnce()
})
