import { Code } from '@connectrpc/connect'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { Stage } from '@/shared/api'
import { createFakeProviderTransport } from '@/test/providers'
import { chooseOption } from '@/test/listbox'
import { createTestQueryClient, withProviders } from '@/test/session'
import { ModelPairForm } from './ModelPairForm'

afterEach(() => initializeI18n('ko'))

const MODELS = [
  { providerId: 'openrouter', modelId: 'writer-a', label: 'Writer A' },
  { providerId: 'openrouter', modelId: 'writer-b', label: 'Writer B' },
  { providerId: 'openrouter', modelId: 'writer-c', label: 'Writer C' },
]

describe('ModelPairForm structured failures', () => {
  it.each([
    {
      locale: 'ko' as const,
      activeMessage: '비활성화된 모델이에요.',
      pairMessage: '서로 다른 모델을 선택해 주세요.',
    },
    {
      locale: 'en' as const,
      activeMessage: 'That model is disabled.',
      pairMessage: 'Select two different models.',
    },
  ])(
    'renders active and comparison-pair reasons without backend prose in $locale',
    async ({ locale, activeMessage, pairMessage }) => {
      initializeI18n(locale)
      const user = userEvent.setup()
      const transport = createFakeProviderTransport({
        models: MODELS,
        selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer-a' }],
        saveFailure: { reason: 'MODEL_DISABLED', code: Code.FailedPrecondition },
        savePairFailure: { reason: 'MODEL_CANDIDATES_DUPLICATE', code: Code.InvalidArgument },
      })
      render(<ModelPairForm stage="write" />, {
        wrapper: withProviders(transport, createTestQueryClient()),
      })

      const selects = await screen.findAllByRole('combobox')
      await waitFor(() => expect(selects[0]).toHaveTextContent('Writer A'))

      await chooseOption(user, selects[0], 'Writer C')
      expect(await screen.findByText(activeMessage)).toBeInTheDocument()
      expect(selects[0]).toHaveAttribute('aria-invalid', 'true')
      expect(selects[0]).toHaveAccessibleDescription(activeMessage)

      await chooseOption(user, selects[1], 'Writer A')
      await chooseOption(user, selects[2], 'Writer B')
      await user.click(screen.getByRole('button'))

      expect(await screen.findByText(pairMessage)).toBeInTheDocument()
      expect(document.body).not.toHaveTextContent('private backend prose')
      expect(document.body).not.toHaveTextContent('[failed_precondition]')
      expect(document.body).not.toHaveTextContent('[invalid_argument]')
    },
  )
})
