import { create } from '@bufbuild/protobuf'
import { render, screen, within } from '@testing-library/react'
import { expect, it } from 'vitest'
import type { ExperimentCandidate, ModelExperiment } from '@/entities/model-experiment'
import { ObservationSchema } from '@/shared/api'
import { CandidateComparison } from './CandidateComparison'

function written(title: string): ExperimentCandidate['output'] {
  return {
    kind: 'write',
    content: { $typeName: 'postpilot.v1.PostContent', title, summary: '', tags: [], blocks: [] },
  }
}

const base: ModelExperiment = {
  id: 'exp',
  stage: 'write',
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
  source: 'post',
  voicePromptKey: '',
  voicePromptText: '',
  voiceAnswer: '',
  candidates: [
    {
      id: 'right',
      displaySide: 'right',
      status: 'succeeded',
      output: written('오른쪽 결과'),
      badges: [],
      otherNote: '',
      failure: undefined,
      modelLabel: '',
    },
    {
      id: 'left',
      displaySide: 'left',
      status: 'succeeded',
      output: written('왼쪽 결과'),
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
  // One scroller per screen (THEME-25): the panel must never open a nested one, which
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
  // compared without switching (THEME-24).
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
            // Stored in the order the owner ticked them; shown by group (MODEL-62).
            badges: ['other', 'ai_like'],
            otherNote: '제목이 비슷해요',
          },
        ],
      }}
      activeCandidateId="left"
    />,
  )
  expect(screen.getByText('속도가 빨라요')).toBeInTheDocument()
  expect(screen.getByText('문체가 잘 맞아요')).toBeInTheDocument()
  const complaint = screen.getByText('AI 같아요')
  const other = screen.getByText('기타')
  expect(screen.getByText('제목이 비슷해요')).toBeInTheDocument()
  // 기타 is its own group below the negatives, and never in the warning tone.
  expect(complaint.compareDocumentPosition(other) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  expect(other.className).not.toMatch(/warning/)
  expect(complaint.className).toMatch(/warning/)
})

// The lab compares observe and write alone (MODEL-30): an observe candidate is read as the
// observations it made, one entry per photo.
it('renders an observe candidate as its observations', () => {
  render(
    <CandidateComparison
      experiment={{
        ...base,
        stage: 'observe',
        candidates: base.candidates.map((candidate) => ({
          ...candidate,
          output: {
            kind: 'observe',
            observations: [
              create(ObservationSchema, {
                file: 'photo.jpg',
                scene: `${candidate.displaySide} 장면`,
                objects: ['컵'],
              }),
            ],
          },
        })),
      }}
      activeCandidateId="left"
    />,
  )
  const [left] = screen.getAllByRole('article')
  expect(within(left).getByText('photo.jpg')).toBeInTheDocument()
  expect(within(left).getByText('left 장면')).toBeInTheDocument()
  expect(within(left).getByText('컵')).toBeInTheDocument()
})

// MODEL-67: a 말투 반영 비교 shows the owner's answer on top and each piece with its comparison,
// drawn by the slot the page supplies; once purged, the answer says it is gone.
it('shows the owner answer above each piece and its comparison', () => {
  const voiced: ModelExperiment = {
    ...base,
    origin: 'lab',
    source: 'voice',
    voicePromptKey: 'opening_greeting',
    voicePromptText: '첫인사를 써 보세요.',
    voiceAnswer: '안녕하세요, 동네 빵집이에요.',
    candidates: base.candidates.map((candidate) => ({
      ...candidate,
      output: {
        kind: 'voice' as const,
        text: `${candidate.id} 조각`,
        comparison: [
          { item: 'endings' as const, unknown: false, distance: 0.4, headline: '해요', facets: [] },
        ],
      },
    })),
  }
  const { rerender } = render(
    <CandidateComparison
      experiment={voiced}
      activeCandidateId="left"
      renderComparison={(items) => <p>{`비교 ${items.length}개`}</p>}
    />,
  )
  const answer = screen.getByRole('region', { name: '내 답' })
  expect(answer).toHaveTextContent('첫인사를 써 보세요.')
  expect(answer).toHaveTextContent('안녕하세요, 동네 빵집이에요.')
  const panels = screen.getAllByRole('article')
  expect(within(panels[0]!).getByText('left 조각')).toBeInTheDocument()
  expect(within(panels[0]!).getByText('비교 1개')).toBeInTheDocument()
  expect(within(panels[1]!).getByText('right 조각')).toBeInTheDocument()

  rerender(
    <CandidateComparison experiment={{ ...voiced, voiceAnswer: '' }} activeCandidateId="left" />,
  )
  expect(screen.getByRole('region', { name: '내 답' })).toHaveTextContent(
    '보관 기간이 지나 내 답은 지워졌어요.',
  )
})
