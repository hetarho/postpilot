import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { Stage } from '@/shared/api'
import { createFakeProviderTransport } from '@/test/providers'
import { createTestQueryClient, withProviders } from '@/test/session'
import { CandidatePairSelect } from './CandidatePairSelect'

afterEach(() => initializeI18n('ko'))

/** Deliberately handed to the fake worst-first, so a passing assertion is the component's
 *  ordering rather than the order the server happened to send. */
const MODELS = [
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

function renderPair() {
  const transport = createFakeProviderTransport({ models: MODELS })
  render(<CandidatePairSelect stage="write" />, {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
}

describe('CandidatePairSelect levels (T095/MODEL-44)', () => {
  it('grades and orders BOTH fields the same way the stage selector does', async () => {
    const user = userEvent.setup()
    renderPair()

    const triggers = await screen.findAllByRole('combobox')
    expect(triggers).toHaveLength(2)

    for (const trigger of triggers) {
      await waitFor(() => expect(trigger).toBeEnabled())
      await user.click(trigger)
      // Every option IS a model here: this field's placeholder is its empty state, not a
      // listed choice.
      const options = screen.getAllByRole('option').map((option) => option.textContent)
      expect(options).toEqual(['가성비 · Cheap', '최고 · Flagship', 'Ungraded'])
      // Close before opening the neighbour, so the next query cannot read this panel.
      await user.keyboard('{Escape}')
    }
  })

  it('keeps the unaffordable reason readable behind the grade', async () => {
    const user = userEvent.setup()
    const transport = createFakeProviderTransport({
      models: [
        {
          providerId: 'openrouter',
          modelId: 'dear',
          label: 'Dear',
          levels: { [Stage.WRITE]: 'top' as const },
          affordable: false,
          requiredCredits: 40,
        },
      ],
    })
    render(<CandidatePairSelect stage="write" />, {
      wrapper: withProviders(transport, createTestQueryClient()),
    })

    const trigger = (await screen.findAllByRole('combobox'))[0]
    await waitFor(() => expect(trigger).toBeEnabled())
    await user.click(trigger)
    // The grade leads, the model, then the reason it cannot be picked — none of the three
    // displaces the others, and the grade never makes an unusable model look usable.
    const option = screen.getAllByRole('option')[0]
    expect(option.textContent).toMatch(/^최고 · Dear \(/)
    expect(option).toHaveAttribute('aria-disabled', 'true')
  })
})
