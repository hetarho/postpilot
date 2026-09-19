import type { ReactNode } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, it, vi } from 'vitest'
import { VoiceRuleComparisonPage } from './VoiceRuleComparisonPage'

const mocks = vi.hoisted(() => ({
  decide: vi.fn(),
  routeVoiceId: 'voice-default',
  comparisonVoiceId: 'voice-default',
}))

vi.mock('@tanstack/react-router', () => ({
  useParams: () => ({ voiceId: mocks.routeVoiceId, id: 'comparison-1' }),
  Link: ({ children }: { children: ReactNode }) => (
    <a href="/voices/voice-default/rules">{children}</a>
  ),
}))

vi.mock('@/entities/generation-job', () => ({ useJob: () => ({}) }))
vi.mock('@/entities/session', () => ({ useSession: () => ({ user: { id: 'alice' } }) }))
// The comparison query, the decide/retry calls and the invalidation they own belong to the voice
// entity (ARCH-17); the screen is exercised against that one hook.
vi.mock('@/entities/voice', () => ({
  useVoiceRuleComparison: () => ({
    queryKey: ['comparison'],
    comparison: {
      id: 'comparison-1',
      voiceId: mocks.comparisonVoiceId,
      status: 'review',
      jobId: 'job-1',
      chosenSide: '',
      candidates: [
        { id: 'candidate-a', side: 'A', output: 'A 결과', status: 'succeeded', error: '' },
        { id: 'candidate-b', side: 'B', output: 'B 결과', status: 'succeeded', error: '' },
      ],
    },
    isPending: false,
    isError: false,
    refetch: vi.fn(),
    decidePending: false,
    decide: mocks.decide,
    retryPending: false,
    retry: vi.fn(),
  }),
}))

beforeEach(() => {
  mocks.decide.mockReset().mockResolvedValue({})
  mocks.routeVoiceId = 'voice-default'
  mocks.comparisonVoiceId = 'voice-default'
})

it('keeps the desktop selector visible and submits candidate B', async () => {
  const user = userEvent.setup()
  render(<VoiceRuleComparisonPage />)

  const selector = screen.getByRole('tablist', { name: '선택할 후보' })
  expect(selector).not.toHaveClass('md:hidden')
  await user.click(screen.getByRole('tab', { name: 'B' }))
  await user.click(screen.getByRole('button', { name: '이 글이 더 나아요' }))

  expect(mocks.decide).toHaveBeenCalledWith('B')
})

it('refuses a comparison that belongs to a different voice than the route', () => {
  mocks.routeVoiceId = 'voice-other'
  render(<VoiceRuleComparisonPage />)

  expect(screen.getByRole('alert')).toHaveTextContent('다른 말투의 기록')
  expect(screen.queryByRole('button', { name: '이 글이 더 나아요' })).not.toBeInTheDocument()
})
