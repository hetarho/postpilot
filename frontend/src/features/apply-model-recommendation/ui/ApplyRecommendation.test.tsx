import { Code } from '@connectrpc/connect'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import type { RecommendationSet } from '@/entities/model-catalog'
import { Stage } from '@/shared/api'
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

const ref = (modelId: string) => ({ providerId: 'openrouter', modelId })

/** A set as the server ships it (MODEL-23): observe and write carry their active model and an
 *  A/B pair, analyze its active model alone — seven refs. */
const sevenRefSet: RecommendationSet = {
  id: 'balanced-2026-08',
  label: 'Balanced',
  selections: [
    {
      stage: 'observe',
      active: ref('observe-active'),
      candidateA: ref('observe-a'),
      candidateB: ref('observe-b'),
    },
    { stage: 'analyze', active: ref('analyze-active') },
    {
      stage: 'write',
      active: ref('write-active'),
      candidateA: ref('write-a'),
      candidateB: ref('write-b'),
    },
  ],
}

describe('ApplyRecommendation over a seven-model set', () => {
  it('says it saves seven models at once', async () => {
    render(<ApplyRecommendation recommendation={sevenRefSet} />, {
      wrapper: withProviders(createFakeProviderTransport(), createTestQueryClient()),
    })
    expect(
      screen.getByText(/세 단계의 활성 모델과 관찰·글 작성 A\/B 쌍, 모델 7개를 한 번에 저장합니다/),
    ).toBeInTheDocument()
  })

  // A set is applied whole, so every one of its seven refs is asked about — and the analyze
  // stage, which has no pair, adds its active model and nothing else.
  it('names every one of the seven refs the balance cannot cover, in set order', async () => {
    const models = [
      'observe-active',
      'observe-a',
      'observe-b',
      'analyze-active',
      'write-active',
      'write-a',
      'write-b',
    ].map((modelId) => ({
      ...ref(modelId),
      stages: [Stage.OBSERVE, Stage.ANALYZE, Stage.WRITE],
      affordable: false,
    }))
    render(<ApplyRecommendation recommendation={sevenRefSet} />, {
      wrapper: withProviders(createFakeProviderTransport({ models }), createTestQueryClient()),
    })
    expect(await screen.findByRole('status')).toHaveTextContent(
      [
        'openrouter/observe-active',
        'openrouter/observe-a',
        'openrouter/observe-b',
        'openrouter/analyze-active',
        'openrouter/write-active',
        'openrouter/write-a',
        'openrouter/write-b',
      ].join(', ') + ' 모델을 쓰려면 크레딧이 부족해요.',
    )
    expect(screen.getByRole('button', { name: '추천 조합 적용' })).toBeDisabled()
  })
})
