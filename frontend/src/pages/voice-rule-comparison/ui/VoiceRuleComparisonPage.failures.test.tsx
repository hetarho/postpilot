import type { ReactNode } from 'react'
import { Code } from '@connectrpc/connect'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { connectAppError } from '@/test/app-error'
import { VoiceRuleComparisonPage } from './VoiceRuleComparisonPage'

const mocks = vi.hoisted(() => ({
  decide: vi.fn(),
  retry: vi.fn(),
  refetch: vi.fn(),
  decideError: undefined as unknown,
  retryError: undefined as unknown,
  status: 'review',
}))

vi.mock('@tanstack/react-router', () => ({
  useParams: () => ({ voiceId: 'voice-default', id: 'comparison-1' }),
  Link: ({ children }: { children: ReactNode }) => (
    <a href="/voices/voice-default/rules">{children}</a>
  ),
}))

vi.mock('@/entities/generation-job', () => ({ useJob: () => ({}) }))
vi.mock('@/entities/session', () => ({ useSession: () => ({ user: { id: 'alice' } }) }))
// The decide and retry calls, their failures and the caches they invalidate are the voice
// entity's (ARCH-17); what this screen owes the user is the localized refusal.
vi.mock('@/entities/voice', async () => {
  const { appFailureFromConnect } = await import('@/shared/api')
  return {
    useVoiceRuleComparison: () => ({
      queryKey: ['comparison'],
      comparison: {
        id: 'comparison-1',
        voiceId: 'voice-default',
        status: mocks.status,
        jobId: 'job-1',
        chosenSide: '',
        candidates: [
          { id: 'candidate-a', side: 'A', output: 'A result', status: 'succeeded' },
          { id: 'candidate-b', side: 'B', output: 'B result', status: 'succeeded' },
        ],
      },
      isPending: false,
      isError: false,
      refetch: mocks.refetch,
      decidePending: false,
      decideFailure: mocks.decideError
        ? appFailureFromConnect(mocks.decideError as Error)
        : undefined,
      decide: mocks.decide,
      retryPending: false,
      retryFailure: mocks.retryError ? appFailureFromConnect(mocks.retryError as Error) : undefined,
      retry: mocks.retry,
    }),
  }
})

beforeEach(() => {
  mocks.decide.mockReset()
  mocks.retry.mockReset()
  mocks.refetch.mockReset().mockResolvedValue(undefined)
  mocks.decideError = undefined
  mocks.retryError = undefined
  mocks.status = 'review'
})

afterEach(() => {
  cleanup()
  initializeI18n('ko')
})

describe('VoiceRuleComparisonPage mutation failures', () => {
  it.each([
    {
      locale: 'ko' as const,
      action: '이 글이 더 나아요',
      message: '현재 말투 상태에서는 이 작업을 할 수 없어요.',
    },
    {
      locale: 'en' as const,
      action: 'This version is better',
      message: "This action is not available in the voice's current state.",
    },
  ])(
    'localizes a decide conflict without raw prose in $locale',
    async ({ locale, action, message }) => {
      initializeI18n(locale)
      const failure = connectAppError('VOICE_INVALID_LIFECYCLE', Code.FailedPrecondition)
      mocks.decideError = failure
      mocks.decide.mockRejectedValue(failure)
      const user = userEvent.setup()
      render(<VoiceRuleComparisonPage />)

      await user.click(screen.getByRole('button', { name: action }))

      expect(await screen.findByRole('alert')).toHaveTextContent(message)
      expect(document.body).not.toHaveTextContent('private backend prose')
      expect(document.body).not.toHaveTextContent('[failed_precondition]')
    },
  )

  it.each([
    {
      locale: 'ko' as const,
      action: '실패한 후보 다시 만들기',
      message: '말투 비교를 찾을 수 없어요.',
    },
    {
      locale: 'en' as const,
      action: 'Regenerate failed candidate',
      message: 'Could not find the voice comparison.',
    },
  ])(
    'localizes a retry refusal without raw prose in $locale',
    async ({ locale, action, message }) => {
      initializeI18n(locale)
      mocks.status = 'failed'
      const failure = connectAppError('VOICE_COMPARISON_NOT_FOUND', Code.NotFound)
      mocks.retryError = failure
      mocks.retry.mockRejectedValue(failure)
      const user = userEvent.setup()
      render(<VoiceRuleComparisonPage />)

      await user.click(screen.getByRole('button', { name: action }))

      expect(await screen.findByRole('alert')).toHaveTextContent(message)
      expect(mocks.refetch).not.toHaveBeenCalled()
      expect(document.body).not.toHaveTextContent('private backend prose')
      expect(document.body).not.toHaveTextContent('[not_found]')
    },
  )
})
