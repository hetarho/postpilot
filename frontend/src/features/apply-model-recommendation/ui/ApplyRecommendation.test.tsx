import { Code } from '@connectrpc/connect'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { createFakeProviderTransport } from '@/test/providers'
import { createTestQueryClient, withProviders } from '@/test/session'
import { ApplyRecommendation } from './ApplyRecommendation'

afterEach(() => initializeI18n('ko'))

describe('ApplyRecommendation structured failure', () => {
  it.each([
    { locale: 'ko' as const, message: '모델 추천 조합을 찾을 수 없어요.' },
    { locale: 'en' as const, message: 'Could not find the model recommendation.' },
  ])('renders the stable reason without backend prose in $locale', async ({ locale, message }) => {
    initializeI18n(locale)
    const user = userEvent.setup()
    const transport = createFakeProviderTransport({
      applyRecommendationFailure: {
        reason: 'MODEL_RECOMMENDATION_NOT_FOUND',
        code: Code.NotFound,
      },
    })
    render(
      <ApplyRecommendation
        recommendation={{ id: 'missing-set', label: 'Balanced', selections: [] }}
      />,
      { wrapper: withProviders(transport, createTestQueryClient()) },
    )

    await user.click(screen.getByRole('button'))

    expect(await screen.findByRole('alert')).toHaveTextContent(message)
    expect(document.body).not.toHaveTextContent('private backend prose')
    expect(document.body).not.toHaveTextContent('[not_found]')
  })
})
