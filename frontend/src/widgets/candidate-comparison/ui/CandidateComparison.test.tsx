import { render, screen } from '@testing-library/react'
import { expect, it } from 'vitest'
import type { ModelExperiment } from '@/entities/model-experiment'
import { CandidateComparison } from './CandidateComparison'

const base: ModelExperiment = {
  id: 'exp',
  stage: 'analyze',
  origin: 'editor',
  status: 'review',
  postSlug: '',
  voiceId: '',
  templateName: '',
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
      badges: [],
      otherNote: '',
      failure: undefined,
      modelLabel: '',
    },
    {
      id: 'left',
      displaySide: 'left',
      status: 'succeeded',
      output: { kind: 'analyze', styleguide: '왼쪽 결과' },
      badges: [],
      otherNote: '',
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

// Once the blind is lifted, what the verdict said about each candidate is read beside the
// model it was said about: the reason one result won is only legible next to the reason the
// other lost.
it('states each revealed candidate its own badges and note', () => {
  render(
    <CandidateComparison
      experiment={{
        ...base,
        status: 'decided',
        revealed: true,
        winnerCandidateId: 'left',
        candidates: [
          {
            ...base.candidates[0],
            modelLabel: 'A model',
            badges: ['fast', 'in_voice'],
            otherNote: '',
          },
          {
            ...base.candidates[1],
            modelLabel: 'B model',
            badges: ['ai_like', 'other'],
            otherNote: '제목이 비슷해요',
          },
        ],
      }}
      activeCandidateId="left"
    />,
  )
  expect(screen.getByText('속도가 빨라요')).toBeInTheDocument()
  expect(screen.getByText('문체가 잘 맞아요')).toBeInTheDocument()
  expect(screen.getByText('AI 같아요')).toBeInTheDocument()
  expect(screen.getByText('제목이 비슷해요')).toBeInTheDocument()
})
