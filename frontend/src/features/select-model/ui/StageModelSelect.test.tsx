import { afterEach, describe, expect, it } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { Code } from '@connectrpc/connect'
import { Stage } from '@/shared/api'
import { createTestQueryClient, withProviders } from '@/test/session'
import { type FakeProvidersOptions, createFakeProviderTransport } from '@/test/providers'
import { StageModelSelect } from './StageModelSelect'

/** The app-drawn listbox renders its options only while it is open, so every assertion about an
 *  option opens the panel first (the native select rendered them all along). The name is a
 *  pattern because a WAI-APG select-only combobox names itself "<label> <current value>". */
async function openPanel(user: ReturnType<typeof userEvent.setup>, name: RegExp) {
  const trigger = await screen.findByRole('combobox', { name })
  await waitFor(() => expect(trigger).toBeEnabled())
  await user.click(trigger)
  return trigger
}

afterEach(() => initializeI18n('ko'))

const MODELS: FakeProvidersOptions['models'] = [
  { providerId: 'openrouter', modelId: 'openrouter/free', label: 'Free', vision: true },
  { providerId: 'openrouter', modelId: 'writer', label: 'Writer', structuredOutput: true },
  {
    providerId: 'anthropic',
    modelId: 'claude',
    label: 'Claude',
    vision: true,
    structuredOutput: true,
    disabledReason: 'API key not configured',
  },
]

function renderSelect(stage: 'observe' | 'write' | 'analyze', options: FakeProvidersOptions = {}) {
  const transport = createFakeProviderTransport({ models: MODELS, ...options })
  const queryClient = createTestQueryClient()
  render(<StageModelSelect stage={stage} />, { wrapper: withProviders(transport, queryClient) })
  return { transport, queryClient }
}

describe('StageModelSelect', () => {
  // AC7: no default — the placeholder is selected and nothing was saved.
  it('starts empty for a fresh account', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderSelect('write', { calls })

    const trigger = await openPanel(user, /작성 모델/)
    expect(trigger).toHaveTextContent('모델을 선택하세요')
    expect(
      screen.getByRole('option', { name: '모델을 선택하세요', selected: true }),
    ).toBeInTheDocument()
    expect(calls).not.toContain('SaveSelection')
  })

  // AC3: observe lists vision models only, with badges.
  it('lists only vision models for observe and badges what each can do', async () => {
    const user = userEvent.setup()
    renderSelect('observe')

    await openPanel(user, /관찰 모델/)
    expect(screen.getByRole('option', { name: 'Free 👁' })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: /Writer/ })).not.toBeInTheDocument()
    expect(
      screen.getByRole('option', { name: 'Claude 👁 구조화 응답 (API key not configured)' }),
    ).toHaveAttribute('aria-disabled', 'true')
  })

  // AC2: a provider without a key is greyed with the exact reason and cannot be picked.
  it('greys a model whose provider has no key, with the reason', async () => {
    const user = userEvent.setup()
    renderSelect('write')

    await openPanel(user, /작성 모델/)
    const option = screen.getByRole('option', { name: /Claude/ })
    expect(option).toHaveAttribute('aria-disabled', 'true')
    expect(option).toHaveTextContent('API key not configured')
  })

  // AC4 (client half): picking saves, and the choice is shown at once.
  it('saves a pick and shows it as selected', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderSelect('write', { calls })

    const trigger = await openPanel(user, /작성 모델/)
    await user.click(screen.getByRole('option', { name: 'Writer 구조화 응답' }))

    await waitFor(() => expect(trigger).toHaveTextContent('Writer'))
    expect(calls).toContain('SaveSelection')
  })

  it('restores a saved choice', async () => {
    renderSelect('write', {
      selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
    })

    const trigger = await screen.findByRole('combobox', { name: /작성 모델/ })
    await waitFor(() => expect(trigger).toHaveTextContent('Writer'))
  })

  it.each([
    { locale: 'ko' as const, message: '네트워크에 연결할 수 없어요.' },
    { locale: 'en' as const, message: 'Could not connect to the network.' },
  ])(
    'associates a structured failed save with the select in $locale',
    async ({ locale, message }) => {
      initializeI18n(locale)
      const user = userEvent.setup()
      renderSelect('write', { saveFails: true })

      const select = await screen.findByRole('combobox')
      await waitFor(() => expect(select).toBeEnabled())
      await user.click(select)
      await user.click(screen.getByRole('option', { name: /Writer/ }))

      expect(await screen.findByRole('alert')).toHaveTextContent(message)
      expect(select).toHaveAttribute('aria-invalid', 'true')
      expect(select).toHaveAccessibleDescription(message)
      expect(document.body).not.toHaveTextContent('private backend prose')
      expect(document.body).not.toHaveTextContent('[unavailable]')
    },
  )

  // AC5: a vanished model is shown greyed with the reason and counts as unselected.
  it('shows a vanished saved model greyed with the reason and treats the stage as unselected', async () => {
    const user = userEvent.setup()
    renderSelect('write', {
      selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'gone' }],
    })

    // The field points at the greyed entry, not at any usable model.
    const trigger = await screen.findByRole('combobox', { name: '작성 모델 openrouter/gone' })
    // The option text is the choice, not the explanation: this entry is the closed control's own
    // value, so the reason is rendered under the field where it cannot be truncated away.
    expect(trigger).toHaveAccessibleDescription('등록된 모델 목록에서 사라졌어요')

    await user.click(trigger)
    expect(screen.getByRole('option', { name: /openrouter\/gone/ })).toHaveAttribute(
      'aria-disabled',
      'true',
    )
  })

  it('says so when the catalog cannot be loaded, without calling a saved model vanished', async () => {
    renderSelect('write', {
      listFails: true,
      selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
    })

    expect(await screen.findByRole('alert')).toHaveTextContent('모델 목록을 불러오지 못했어요')
    const trigger = screen.getByRole('combobox', { name: /작성 모델/ })
    expect(trigger).toHaveAttribute('aria-invalid', 'true')
    expect(trigger).toHaveAccessibleDescription('모델 목록을 불러오지 못했어요.')
    expect(screen.queryByRole('option', { name: /사라졌어요/ })).not.toBeInTheDocument()
  })

  // A model saved for observe that the yaml no longer marks as vision is unusable there.
  it('greys a saved model the stage can no longer use', async () => {
    const user = userEvent.setup()
    renderSelect('observe', {
      selections: [{ stage: Stage.OBSERVE, providerId: 'openrouter', modelId: 'writer' }],
    })

    const trigger = await screen.findByRole('combobox', { name: '관찰 모델 openrouter/writer' })
    expect(trigger).toHaveAccessibleDescription('이 단계에서는 쓸 수 없는 모델이에요')

    await user.click(trigger)
    expect(screen.getByRole('option', { name: /openrouter\/writer/ })).toHaveAttribute(
      'aria-disabled',
      'true',
    )
  })
})

describe('a model above the account tier', () => {
  // A6: it stays listed as upsell, disabled and labelled with the tier that unlocks it —
  // vanishing would teach the user nothing about why the model is not there.
  it('is listed, disabled, and says what it would cost', async () => {
    renderSelect('write', {
      models: [
        { providerId: 'openrouter', modelId: 'writer', label: 'Writer' },
        {
          providerId: 'openrouter',
          modelId: 'anthropic/claude-opus-5',
          label: 'Claude Opus 5',
          requiredCredits: 79,
          affordable: false,
        },
      ],
    })

    const user = userEvent.setup()
    await openPanel(user, /작성 모델/)

    const unaffordable = screen.getByRole('option', { name: 'Claude Opus 5 (크레딧 79 필요)' })
    expect(unaffordable).toHaveAttribute('aria-disabled', 'true')
    expect(screen.getByRole('option', { name: 'Writer' })).not.toHaveAttribute('aria-disabled')
  })

  // A balance is temporary state the next top-up clears, so an unaffordable saved choice is
  // never reported as vanished — and its row is never touched.
  it('keeps a saved choice the balance cannot currently cover', async () => {
    renderSelect('write', {
      models: [
        { providerId: 'openrouter', modelId: 'writer', label: 'Writer' },
        {
          providerId: 'openrouter',
          modelId: 'premium',
          label: 'Premium',
          requiredCredits: 79,
          affordable: false,
        },
      ],
      selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'premium' }],
    })

    // The saved choice is still the field's value, carrying what it would cost rather than
    // being reported as gone.
    expect(await screen.findByText('Premium (크레딧 79 필요)')).toBeInTheDocument()
    expect(screen.queryByText('더 이상 등록되지 않은 모델이에요.')).not.toBeInTheDocument()
  })

  // The client's rendering is never the gate: a save that reaches the server anyway is
  // refused, and the refusal is what the field reports.
  it('reports the server’s refusal rather than predicting it', async () => {
    const user = userEvent.setup()
    renderSelect('write', {
      models: [
        { providerId: 'openrouter', modelId: 'writer', label: 'Writer' },
        {
          providerId: 'openrouter',
          modelId: 'premium',
          label: 'Premium',
          requiredCredits: 79,
          affordable: false,
        },
      ],
      saveFailure: {
        reason: 'INSUFFICIENT_CREDITS',
        code: Code.ResourceExhausted,
        params: {
          required: '79',
          balance: '12',
          renews_at: '2026-10-01T00:00:00Z',
        },
      },
    })

    await openPanel(user, /작성 모델/)
    await user.click(screen.getByRole('option', { name: 'Writer' }))

    expect(await screen.findByText(/크레딧이 79 필요한데 12만 남았어요/)).toBeInTheDocument()
  })
})
