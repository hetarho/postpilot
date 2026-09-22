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
import { ActiveModelForm } from './ActiveModelForm'

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
      render(
        <>
          <ActiveModelForm stage="write" />
          <ModelPairForm stage="write" />
        </>,
        {
          wrapper: withProviders(transport, createTestQueryClient()),
        },
      )

      const selects = await screen.findAllByRole('combobox')
      await waitFor(() => expect(selects[0]).toHaveTextContent('Writer A'))

      await chooseOption(user, selects[0], 'Writer C')
      expect(await screen.findByText(activeMessage)).toBeInTheDocument()
      expect(selects[0]).toHaveAttribute('aria-invalid', 'true')
      expect(selects[0]).toHaveAccessibleDescription(activeMessage)

      // Completing the pair writes it; there is no separate save to press (MODEL-65).
      await chooseOption(user, selects[1], 'Writer A')
      await chooseOption(user, selects[2], 'Writer B')

      expect(await screen.findByText(pairMessage)).toBeInTheDocument()
      // The refusal belongs to the candidate that was just changed.
      expect(selects[2]).toHaveAccessibleDescription(pairMessage)
      expect(selects[1]).not.toHaveAccessibleDescription(pairMessage)
      expect(document.body).not.toHaveTextContent('private backend prose')
      expect(document.body).not.toHaveTextContent('[failed_precondition]')
      expect(document.body).not.toHaveTextContent('[invalid_argument]')
    },
  )
})

describe('ModelPairForm levels (T095/MODEL-44)', () => {
  const GRADED = [
    {
      providerId: 'openrouter',
      modelId: 'flagship',
      label: 'Flagship',
      levels: { [Stage.WRITE]: 'top' as const },
    },
    { providerId: 'openrouter', modelId: 'ungraded', label: 'Ungraded' },
    {
      providerId: 'openrouter',
      modelId: 'cheap',
      label: 'Cheap',
      levels: { [Stage.WRITE]: 'value' as const },
    },
  ]

  // All three fields — the active model and both candidates — read the same list, so all
  // three grade and order it identically. A picker that disagreed with its neighbour would
  // read as a bug in the catalog rather than as a difference in the field.
  it('grades and orders the active field and both candidate fields alike', async () => {
    const user = userEvent.setup()
    const transport = createFakeProviderTransport({ models: GRADED })
    render(
      <>
        <ActiveModelForm stage="write" />
        <ModelPairForm stage="write" />
      </>,
      {
        wrapper: withProviders(transport, createTestQueryClient()),
      },
    )

    const selects = await screen.findAllByRole('combobox')
    expect(selects).toHaveLength(3)
    for (const select of selects) {
      await waitFor(() => expect(select).toBeEnabled())
      await user.click(select)
      const options = screen
        .getAllByRole('option')
        // This form DOES list its placeholder as a choice.
        .slice(1)
        .map((option) => option.textContent)
      expect(options).toEqual(['가성비 · Cheap', '최고 · Flagship', 'Ungraded'])
      await user.keyboard('{Escape}')
    }
  })
})

describe('ModelPairForm saves as it is chosen (MODEL-65)', () => {
  const SAVED = [{ stage: Stage.WRITE, candidateA: MODELS[0], candidateB: MODELS[1] }]

  it('writes the pair the moment a change completes it', async () => {
    const user = userEvent.setup()
    const pairs: Array<{ a: string; b: string }> = []
    const transport = createFakeProviderTransport({
      models: MODELS,
      comparisonPairs: SAVED,
      onSavePair: (pair) => pairs.push(pair),
    })
    render(<ModelPairForm stage="write" />, {
      wrapper: withProviders(transport, createTestQueryClient()),
    })
    // Re-queried rather than held: the form remounts when its saved pair arrives, so a node
    // captured before that is detached.
    await waitFor(() => expect(screen.getAllByRole('combobox')[0]).toHaveTextContent('Writer A'))
    expect(screen.queryByRole('button', { name: 'A/B 조합 저장' })).not.toBeInTheDocument()

    await chooseOption(user, screen.getAllByRole('combobox')[0], 'Writer C')
    await waitFor(() => expect(pairs).toEqual([{ a: 'writer-c', b: 'writer-b' }]))
  })

  // A refusal belongs to the write that earned it. A later change that writes nothing has
  // nothing to be refused, so the message goes rather than following the cursor.
  it('drops a refusal once a change writes nothing', async () => {
    const user = userEvent.setup()
    const transport = createFakeProviderTransport({
      models: MODELS,
      comparisonPairs: SAVED,
      savePairFailure: { reason: 'MODEL_CANDIDATES_DUPLICATE', code: Code.InvalidArgument },
    })
    render(<ModelPairForm stage="write" />, {
      wrapper: withProviders(transport, createTestQueryClient()),
    })
    await waitFor(() => expect(screen.getAllByRole('combobox')[0]).toHaveTextContent('Writer A'))
    await chooseOption(user, screen.getAllByRole('combobox')[0], 'Writer C')
    expect(await screen.findByText('서로 다른 모델을 선택해 주세요.')).toBeInTheDocument()

    // Clearing a side writes nothing.
    await chooseOption(user, screen.getAllByRole('combobox')[1], '모델을 선택하세요')
    await waitFor(() =>
      expect(screen.queryByText('서로 다른 모델을 선택해 주세요.')).not.toBeInTheDocument(),
    )
  })

  it('writes nothing while the pair is incomplete or names one model twice', async () => {
    const user = userEvent.setup()
    const pairs: Array<{ a: string; b: string }> = []
    const transport = createFakeProviderTransport({
      models: MODELS,
      onSavePair: (pair) => pairs.push(pair),
    })
    render(<ModelPairForm stage="write" />, {
      wrapper: withProviders(transport, createTestQueryClient()),
    })
    await waitFor(() => expect(screen.getAllByRole('combobox')[0]).toBeEnabled())

    // One side alone is not a pair.
    await chooseOption(user, screen.getAllByRole('combobox')[0], 'Writer A')
    expect(pairs).toEqual([])

    // The same model twice is refused here, so the stored pair is left alone.
    await chooseOption(user, screen.getAllByRole('combobox')[1], 'Writer A')
    expect(await screen.findByText('서로 다른 모델을 선택해 주세요.')).toBeInTheDocument()
    expect(pairs).toEqual([])

    // Making them distinct completes it.
    await chooseOption(user, screen.getAllByRole('combobox')[1], 'Writer B')
    await waitFor(() => expect(pairs).toEqual([{ a: 'writer-a', b: 'writer-b' }]))
  })
})
