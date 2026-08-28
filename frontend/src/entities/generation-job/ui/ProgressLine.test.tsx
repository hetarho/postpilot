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
  error: '',
  postSlug: 'post-a',
  observeModel: undefined,
  writeModel: undefined,
  createdAt: '',
  updatedAt: '',
}

it('renders the stage-specific progress copy as a live status', () => {
  render(<ProgressLine job={JOB} />)
  expect(screen.getByRole('status')).toHaveTextContent('사진 3/8 관찰됨')
})

it('shows the stored failure and delegates retry to its caller', async () => {
  const retry = vi.fn()
  render(<FailureNotice error="daily quota exhausted" onRetry={retry} />)
  expect(screen.getByRole('alert')).toHaveTextContent('daily quota exhausted')
  await userEvent.click(screen.getByRole('button', { name: '다시 시도' }))
  expect(retry).toHaveBeenCalledOnce()
})
