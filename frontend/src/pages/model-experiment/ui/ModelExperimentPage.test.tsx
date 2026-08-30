import type { ReactNode } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, it, vi } from 'vitest'
import type { ModelExperiment } from '@/entities/model-experiment'
import { ModelExperimentPage } from './ModelExperimentPage'

const mocks = vi.hoisted(() => ({ useExperiment: vi.fn() }))

vi.mock('@tanstack/react-router', () => ({
  useParams: () => ({ id: 'experiment-1' }),
  Link: ({
    children,
    to,
    params,
  }: {
    children: ReactNode
    to: string
    params?: Record<string, string>
  }) => (
    <a href={Object.entries(params ?? {}).reduce((href, [k, v]) => href.replace(`$${k}`, v), to)}>
      {children}
    </a>
  ),
}))

vi.mock('@/entities/model-experiment', () => ({
  useExperiment: mocks.useExperiment,
}))

vi.mock('@/entities/session', () => ({ useSession: () => ({ user: { id: 'alice' } }) }))
vi.mock('@/entities/voice', () => ({
  useVoices: () => ({
    voices: [{ id: 'voice-default', name: '기본 말투', isDefault: true, deleted: false }],
  }),
  voiceRefLabel: (voice: { name: string; deleted: boolean }) =>
    voice.deleted ? `삭제된 말투 · ${voice.name}` : voice.name,
}))

vi.mock('@/features/review-model-experiment', () => ({
  hasExperimentActions: () => true,
  ExperimentActions: ({ activeCandidateId }: { activeCandidateId: string }) => (
    <output aria-label="결정 대상">{activeCandidateId}</output>
  ),
}))

const experiment: ModelExperiment = {
  id: 'experiment-1',
  stage: 'analyze',
  status: 'review',
  postSlug: '',
  voiceId: 'voice-default',
  purposeName: '',
  jobId: 'job-1',
  candidates: [
    {
      id: 'candidate-b',
      displaySide: 'right',
      status: 'succeeded',
      output: { kind: 'analyze', styleguide: 'B 결과' },
      error: '',
      modelLabel: '',
    },
    {
      id: 'candidate-a',
      displaySide: 'left',
      status: 'succeeded',
      output: { kind: 'analyze', styleguide: 'A 결과' },
      error: '',
      modelLabel: '',
    },
  ],
  winnerCandidateId: '',
  outcome: '',
  applyError: '',
  appliedAt: '',
  adoptionRequested: false,
  adoptionError: '',
  adoptedAt: '',
  createdAt: '',
  finishedAt: '',
  decidedAt: '',
  revealed: false,
}

beforeEach(() => {
  mocks.useExperiment.mockReturnValue({
    experiment,
    isPending: false,
    isError: false,
    refetch: vi.fn(),
  })
})

it('keeps the choice control visible on desktop and can target candidate B', async () => {
  const user = userEvent.setup()
  render(<ModelExperimentPage />)

  const selector = screen.getByRole('tablist', { name: '선택할 후보' })
  expect(selector).not.toHaveClass('md:hidden')
  expect(screen.getByLabelText('결정 대상')).toHaveTextContent('candidate-a')
  // An analyze experiment names the voice whose corpus it froze.
  expect(screen.getByText('말투 · 기본 말투')).toBeInTheDocument()

  await user.click(screen.getByRole('tab', { name: 'B' }))

  expect(screen.getByLabelText('결정 대상')).toHaveTextContent('candidate-b')
})
