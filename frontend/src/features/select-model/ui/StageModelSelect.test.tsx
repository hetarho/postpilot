import { afterEach, describe, expect, it } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { Code } from '@connectrpc/connect'
import { ProtoPlan, Stage } from '@/shared/api'
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
      screen.getByRole('option', { name: 'Claude 👁 구조화 응답 — API key not configured' }),
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
  it('is listed, disabled, and says which plan it needs', async () => {
    renderSelect('write', {
      models: [
        { providerId: 'openrouter', modelId: 'writer', label: 'Writer' },
        {
          providerId: 'openrouter',
          modelId: 'anthropic/claude-opus-5',
          label: 'Claude Opus 5',
          minPlan: ProtoPlan.MAX,
          locked: true,
        },
      ],
    })

    const user = userEvent.setup()
    await openPanel(user, /작성 모델/)

    const locked = screen.getByRole('option', { name: 'Claude Opus 5 — Max 플랜부터' })
    expect(locked).toHaveAttribute('aria-disabled', 'true')
    expect(screen.getByRole('option', { name: 'Writer' })).not.toHaveAttribute('aria-disabled')
  })

  // A saved choice the tier can no longer run is kept by the server, so the field says which
  // plan restores it rather than "no longer registered" — which would send the user off to
  // pick a replacement they do not need.
  it('explains a saved choice that a downgrade locked, without calling it vanished', async () => {
    renderSelect('write', {
      models: [
        { providerId: 'openrouter', modelId: 'writer', label: 'Writer' },
        {
          providerId: 'openrouter',
          modelId: 'premium',
          label: 'Premium',
          minPlan: ProtoPlan.MAX,
          locked: true,
        },
      ],
      selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'premium' }],
      lockedSelections: true,
    })

    expect(await screen.findByText('Max 플랜부터')).toBeInTheDocument()
    expect(screen.queryByText('더 이상 등록되지 않은 모델이에요.')).not.toBeInTheDocument()
  })

  // The client's rendering is never the gate: a save that reaches the server anyway is
  // refused, and the refusal is what the field reports.
  it('reports the server’s refusal rather than predicting it', async () => {
    const user = userEvent.setup()
    renderSelect('write', {
      models: [
        { providerId: 'openrouter', modelId: 'writer', label: 'Writer' },
        { providerId: 'openrouter', modelId: 'premium', label: 'Premium', locked: true },
      ],
      saveFailure: {
        reason: 'MODEL_LOCKED',
        code: Code.PermissionDenied,
        params: {
          model: 'openrouter/premium',
          models: 'openrouter/premium',
          required_plan: 'max',
        },
      },
    })

    await openPanel(user, /작성 모델/)
    await user.click(screen.getByRole('option', { name: 'Writer' }))

    expect(
      await screen.findByText('openrouter/premium 모델은 Max 플랜부터 쓸 수 있어요.'),
    ).toBeInTheDocument()
  })
})
