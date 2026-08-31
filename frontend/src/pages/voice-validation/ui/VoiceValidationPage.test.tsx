import type { ReactNode } from 'react'
import { Code } from '@connectrpc/connect'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { connectAppError } from '@/test/app-error'
import { VoiceValidationPage } from './VoiceValidationPage'

const mocks = vi.hoisted(() => ({
  retry: vi.fn(),
  refetch: vi.fn(),
  retryError: undefined as unknown,
}))

vi.mock('@tanstack/react-router', () => ({
  useParams: () => ({ voiceId: 'voice-default', id: 'validation-1' }),
  Link: ({ children }: { children: ReactNode }) => (
    <a href="/voices/voice-default/validations">{children}</a>
  ),
}))

vi.mock('@connectrpc/connect-query', () => ({
  useTransport: () => ({}),
  useMutation: () => ({ mutateAsync: mocks.retry, isPending: false, error: mocks.retryError }),
}))

vi.mock('@tanstack/react-query', () => ({
  useQuery: () => ({
    data: {
      validation: {
        id: 'validation-1',
        voiceId: 'voice-default',
        profileVersion: 3n,
        status: 'failed',
        jobId: 'job-1',
        judgeEnabled: false,
        totalCount: 0,
        yCount: 0,
        items: [],
      },
    },
    isPending: false,
    isError: false,
    refetch: mocks.refetch,
  }),
}))

vi.mock('@/entities/generation-job', () => ({ useJob: () => ({}) }))
vi.mock('@/entities/session', () => ({ useSession: () => ({ user: { id: 'alice' } }) }))
vi.mock('@/entities/voice', () => ({
  voiceValidationQueryKey: () => ['validation'],
  voiceValidationState: (value: string) => value,
}))

beforeEach(() => {
  mocks.retry.mockReset()
  mocks.refetch.mockReset().mockResolvedValue(undefined)
  mocks.retryError = undefined
})

afterEach(() => {
  cleanup()
  initializeI18n('ko')
})

describe('VoiceValidationPage retry failure', () => {
  it.each([
    {
      locale: 'ko' as const,
      action: '실패한 항목 다시 실행',
      message: '말투 검증을 찾을 수 없어요.',
      status: '실패',
    },
    {
      locale: 'en' as const,
      action: 'Retry failed items',
      message: 'Could not find the voice validation.',
      status: 'Failed',
    },
  ])(
    'localizes the structured refusal without raw prose in $locale',
    async ({ locale, action, message, status }) => {
      initializeI18n(locale)
      const failure = connectAppError('VOICE_VALIDATION_NOT_FOUND', Code.NotFound)
      mocks.retryError = failure
      mocks.retry.mockRejectedValue(failure)
      const user = userEvent.setup()
      render(<VoiceValidationPage />)

      expect(screen.getByText(status)).toBeInTheDocument()
      expect(screen.queryByText('failed')).not.toBeInTheDocument()

      await user.click(screen.getByRole('button', { name: action }))

      expect(await screen.findByRole('alert')).toHaveTextContent(message)
      expect(mocks.refetch).not.toHaveBeenCalled()
      expect(document.body).not.toHaveTextContent('private backend prose')
      expect(document.body).not.toHaveTextContent('[not_found]')
    },
  )
})
