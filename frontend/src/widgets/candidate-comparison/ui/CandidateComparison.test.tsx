import { render, screen } from '@testing-library/react'
import { expect, it, vi } from 'vitest'
import type { ModelExperiment } from '@/entities/model-experiment'
import { CandidateComparison } from './CandidateComparison'

const base: ModelExperiment = {
  id: 'exp',
  stage: 'analyze',
  status: 'review',
  postSlug: '',
  jobId: 'job',
  winnerCandidateId: '',
  outcome: '',
  applyError: '',
  appliedAt: '',
  createdAt: '',
  finishedAt: '',
  decidedAt: '',
  revealed: false,
  candidates: [
    {
      id: 'right',
      displaySide: 'right',
      status: 'succeeded',
      output: { kind: 'analyze', styleguide: '오른쪽 결과' },
      error: '',
      modelLabel: '',
    },
    {
      id: 'left',
      displaySide: 'left',
      status: 'succeeded',
      output: { kind: 'analyze', styleguide: '왼쪽 결과' },
      error: '',
      modelLabel: '',
    },
  ],
}

it('keeps stable side order and hides model/accounting before reveal', () => {
  render(
    <CandidateComparison
      experiment={base}
      activeCandidateId="left"
      onActiveCandidateChange={vi.fn()}
    />,
  )
  const candidates = screen.getAllByRole('article')
  expect(candidates[0]).toHaveAccessibleName('후보 A')
  expect(candidates[0]).toHaveTextContent('왼쪽 결과')
  expect(screen.queryByText(/secret-provider/)).not.toBeInTheDocument()
  expect(candidates[0]).toHaveClass('overflow-y-auto')
})

it('reveals label, tokens, latency, and estimated cost only after verdict', () => {
  const revealed: ModelExperiment = {
    ...base,
    status: 'decided',
    revealed: true,
    candidates: base.candidates.map((candidate) => ({
      ...candidate,
      model: { providerId: 'p', modelId: candidate.id },
      modelLabel: `모델 ${candidate.id}`,
      usage: {
        promptTokens: 100n,
        completionTokens: 20n,
        costMicrousd: 12n,
        costSource: 'estimated',
        latencyMs: 500n,
      },
    })),
  }
  render(
    <CandidateComparison
      experiment={revealed}
      activeCandidateId="left"
      onActiveCandidateChange={vi.fn()}
    />,
  )
  expect(screen.getByText('모델 left')).toBeInTheDocument()
  expect(screen.getAllByText(/≈ \$0\.000012/)).toHaveLength(2)
})
