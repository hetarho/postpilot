import { render, screen } from '@testing-library/react'
import { expect, it, vi } from 'vitest'
import { hasExperimentActions } from '../model/experiment-actions'
import type { ModelExperiment } from '@/entities/model-experiment'
import { ExperimentActions } from './ExperimentActions'

const actions = vi.hoisted(() => vi.fn())
vi.mock('@/entities/model-experiment', async (original) => ({
  ...(await original<typeof import('@/entities/model-experiment')>()),
  useExperimentActions: actions,
}))
const fixture = {
  id: 'paid-record',
  stage: 'write',
  origin: 'lab',
  status: 'completed',
  reviewMode: 'candidate_ranking',
  appliedAt: '',
  adoptedAt: '',
  applyFailure: undefined,
  adoptionFailure: undefined,
  candidates: [{ id: 'a', status: 'succeeded' }],
} as ModelExperiment

it.each([
  'queued',
  'running',
  'review',
  'partial',
  'completed',
  'decided',
  'dismissed',
  'failed',
] as const)('keeps %s paid results read-only without mounting legacy mutation hooks', (status) => {
  const experiment = { ...fixture, status }
  render(<ExperimentActions experiment={experiment} activeCandidateId="a" />)
  expect(hasExperimentActions(experiment)).toBe(false)
  expect(screen.getByRole('status')).toHaveTextContent('열람과 복사만')
  expect(screen.getByRole('link', { name: '글쓰기 테스트 시작하기' })).toHaveAttribute(
    'href',
    '/tests',
  )
  expect(screen.queryByRole('button')).not.toBeInTheDocument()
  expect(
    screen.queryByText(/순위 정하기|이 결과로 선택|다시 시도|활성 모델로 사용/),
  ).not.toBeInTheDocument()
  expect(actions).not.toHaveBeenCalled()
})

it('retains recorded publication receipts without offering to apply or adopt again', () => {
  render(
    <ExperimentActions
      experiment={{
        ...fixture,
        appliedAt: '2026-10-07T01:00:00Z',
        adoptedAt: '2026-10-07T02:00:00Z',
      }}
      activeCandidateId="a"
    />,
  )
  expect(screen.getByText(/요청한 글 적용 기록/)).toBeInTheDocument()
  expect(screen.getByText(/요청한 모델 변경 기록/)).toBeInTheDocument()
  expect(screen.queryByRole('button')).not.toBeInTheDocument()
})
