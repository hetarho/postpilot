import { render, screen } from '@testing-library/react'
import { expect, it } from 'vitest'
import type { ModelExperiment } from '@/entities/model-experiment'
import { CandidateComparison } from './CandidateComparison'

const base: ModelExperiment = {
  id: 'exp',
  stage: 'analyze',
  status: 'review',
  postSlug: '',
  voiceId: '',
  purposeName: '',
  jobId: 'job',
  winnerCandidateId: '',
  outcome: '',
  applyFailure: undefined,
  appliedAt: '',
  adoptionRequested: false,
  adoptionFailure: undefined,
  adoptedAt: '',
  createdAt: '',
  finishedAt: '',
  decidedAt: '',
  revealed: false,
  targetLanguage: undefined,
  candidates: [
    {
      id: 'right',
      displaySide: 'right',
      status: 'succeeded',
      output: { kind: 'analyze', styleguide: '오른쪽 결과' },
      failure: undefined,
      modelLabel: '',
    },
    {
      id: 'left',
      displaySide: 'left',
      status: 'succeeded',
      output: { kind: 'analyze', styleguide: '왼쪽 결과' },
      failure: undefined,
      modelLabel: '',
    },
  ],
}

it('keeps stable side order and hides model/accounting before reveal', () => {
  render(<CandidateComparison experiment={base} activeCandidateId="left" />)
  const candidates = screen.getAllByRole('article')
  expect(candidates[0]).toHaveAccessibleName('후보 A')
  expect(candidates[0]).toHaveTextContent('왼쪽 결과')
  expect(screen.queryByText(/secret-provider/)).not.toBeInTheDocument()
  // One scroller per screen (design-language §4.4): the panel must never open a nested one, which
  // also reset the reader's position on every A/B switch.
  expect(candidates[0]).not.toHaveClass('overflow-y-auto')
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
  render(<CandidateComparison experiment={revealed} activeCandidateId="left" />)
  expect(screen.getByText('모델 left')).toBeInTheDocument()
  // Both candidates' accounting is on screen at once, outside the panels, so the reveal can be
  // compared without switching (design-language §4.3).
  expect(screen.getAllByText(/≈ \$0\.000012/)).toHaveLength(2)
})
