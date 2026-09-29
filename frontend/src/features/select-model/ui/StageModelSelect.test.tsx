import { afterEach, describe, expect, it } from 'vitest'
import { act, cleanup, render, screen, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { Code } from '@connectrpc/connect'
import { GetMyPlanResponseSchema, Stage } from '@/shared/api'
import { myPlanQueryKey } from '@/entities/plan'
import type { ModelAvailability, ModelRef, ModelVerdict } from '@/entities/model-catalog'
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

describe('StageModelSelect levels (T095/MODEL-44)', () => {
  const GRADED: FakeProvidersOptions['models'] = [
    {
      providerId: 'openrouter',
      modelId: 'flagship',
      label: 'Flagship',
      levels: { [Stage.WRITE]: 'top' },
    },
    { providerId: 'openrouter', modelId: 'ungraded', label: 'Ungraded' },
    {
      providerId: 'openrouter',
      modelId: 'cheap',
      label: 'Cheap',
      levels: { [Stage.WRITE]: 'value' },
    },
  ]

  it('orders 가성비 first with the ungraded models last, and leads each label with its grade', async () => {
    const user = userEvent.setup()
    renderSelect('write', { models: GRADED })

    await openPanel(user, /작성 모델/)
    const options = screen
      .getAllByRole('option')
      // The placeholder is not a model.
      .slice(1)
      .map((option) => option.textContent)
    expect(options).toEqual(['가성비 · Cheap', '최고 · Flagship', 'Ungraded'])
  })

  // A grade is per stage, so the same model can lead one picker and trail another.
  it('reads only its own stage grade', async () => {
    const user = userEvent.setup()
    renderSelect('analyze', {
      models: [
        {
          providerId: 'openrouter',
          modelId: 'split',
          label: 'Split',
          levels: { [Stage.WRITE]: 'top' },
        },
      ],
    })

    await openPanel(user, /문체 분석 모델/)
    expect(screen.getByRole('option', { name: 'Split' })).toBeInTheDocument()
  })
})

describe('StageModelSelect', () => {
  // MODEL-23: no default — the placeholder is selected and nothing was saved.
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

  // MODEL-14, MODEL-15: observe lists vision models only, with badges.
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

  // MODEL-11: a provider without a key is greyed with the exact reason and cannot be picked.
  it('greys a model whose provider has no key, with the reason', async () => {
    const user = userEvent.setup()
    renderSelect('write')

    await openPanel(user, /작성 모델/)
    const option = screen.getByRole('option', { name: /Claude/ })
    expect(option).toHaveAttribute('aria-disabled', 'true')
    expect(option).toHaveTextContent('API key not configured')
  })

  // MODEL-25, client half: picking saves, and the choice is shown at once.
  it('saves a pick and shows it as selected', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderSelect('write', { calls })

    const trigger = await openPanel(user, /작성 모델/)
    await user.click(screen.getByRole('option', { name: 'Writer 구조화 응답' }))

    await waitFor(() => expect(trigger).toHaveTextContent('Writer'))
    expect(calls).toContain('SaveSelection')
  })

  // MODEL-23: analyze is never compared, yet it keeps its one active selection — 말투 만들기 and
  // memory extraction read it — so its picker saves exactly as the other two do.
  it('saves the analyze stage’s active model', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderSelect('analyze', { calls })

    const trigger = await openPanel(user, /문체 분석 모델/)
    await user.click(screen.getByRole('option', { name: /Writer/ }))

    await waitFor(() => expect(trigger).toHaveTextContent('Writer'))
    expect(calls).toContain('SaveSelection')
    expect(calls).not.toContain('SaveComparisonPair')
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

  // MODEL-24: a vanished model is shown greyed with the reason and counts as unselected.
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

describe('graded access (T481)', () => {
  it('rechecks the model projection when a refreshed plan changes its grade ceiling', async () => {
    const user = userEvent.setup()
    const models: FakeProvidersOptions['models'] = [
      {
        providerId: 'openrouter',
        modelId: 'paid',
        label: 'Paid',
        access: {
          [Stage.WRITE]: {
            grade: 'value',
            requiredPlan: 'basic',
            entitled: false,
            unavailableReason: 'MODEL_PLAN_REQUIRED',
          },
        },
      },
    ]
    const transport = createFakeProviderTransport({ models })
    const queryClient = createTestQueryClient()
    render(<StageModelSelect stage="write" />, { wrapper: withProviders(transport, queryClient) })
    await openPanel(user, /작성 모델/)
    expect(screen.getByRole('option', { name: /Paid.*Basic/ })).toHaveAttribute(
      'aria-disabled',
      'true',
    )

    models[0]!.access![Stage.WRITE] = {
      grade: 'value',
      requiredPlan: 'basic',
      entitled: true,
      unavailableReason: '',
    }
    await new Promise((resolve) => setTimeout(resolve, 2))
    act(() =>
      queryClient.setQueryData(myPlanQueryKey(transport), create(GetMyPlanResponseSchema, {})),
    )
    await waitFor(() =>
      expect(screen.getByRole('option', { name: 'Paid' })).not.toHaveAttribute('aria-disabled'),
    )
  })

  it.each([
    {
      locale: 'ko' as const,
      lock: 'Plus 요금제부터 쓸 수 있어요',
      free: '무료 · Free',
      note: /무료 모델은 공급자의/,
    },
    {
      locale: 'en' as const,
      lock: 'Available from the Plus plan',
      free: 'Free · Free',
      note: /Free models may be limited/,
    },
  ])(
    'retains a saved locked model and lets its owner explicitly select a zero-balance free model in $locale',
    async ({ locale, lock, free, note }) => {
      initializeI18n(locale)
      const calls: string[] = []
      const user = userEvent.setup()
      renderSelect('write', {
        calls,
        models: [
          {
            providerId: 'openrouter',
            modelId: 'paid',
            label: 'Paid',
            levels: { [Stage.WRITE]: 'premium' },
            access: {
              [Stage.WRITE]: {
                grade: 'premium',
                requiredPlan: 'plus',
                entitled: false,
                unavailableReason: 'MODEL_PLAN_REQUIRED',
              },
            },
          },
          {
            providerId: 'openrouter',
            modelId: 'free',
            label: 'Free',
            affordable: false,
            levels: { [Stage.WRITE]: 'free' },
            access: {
              [Stage.WRITE]: {
                grade: 'free',
                requiredPlan: 'light',
                entitled: true,
                freePathAvailable: true,
              },
            },
          },
        ],
        selections: [
          {
            stage: Stage.WRITE,
            providerId: 'openrouter',
            modelId: 'paid',
            requiredPlan: 'plus',
            unavailableReason: 'MODEL_PLAN_REQUIRED',
          },
        ],
      })

      const trigger = await screen.findByRole('combobox', { name: /paid/i })
      await waitFor(() => expect(trigger).toHaveAccessibleDescription(lock))
      expect(trigger).toHaveClass('pointer-coarse:min-h-11')
      expect(screen.getByText(note)).toHaveClass('break-words')
      expect(calls).not.toContain('SaveSelection')
      await user.click(trigger)
      expect(screen.getByRole('option', { name: /Paid.*Plus/ })).toHaveAttribute(
        'aria-disabled',
        'true',
      )
      await user.click(screen.getByRole('option', { name: free }))
      await waitFor(() => expect(calls).toContain('SaveSelection'))
      expect(trigger).toHaveTextContent('Free')
    },
  )
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

  // VIDEO-11: watching a clip is a capability of its own, narrower than vision, and the picker
  // says which models have it — a post with a video needs it of the observe model.
  it('badges a model that takes video input', async () => {
    const user = userEvent.setup()
    renderSelect('observe', {
      models: [
        {
          providerId: 'openrouter',
          modelId: 'watcher',
          label: 'Watcher',
          vision: true,
          videoInput: true,
          signedVideoUrl: false,
          inlineStaticVideo: true,
        },
        { providerId: 'openrouter', modelId: 'blind', label: 'Blind', vision: true },
      ],
    })

    await openPanel(user, /관찰 모델/)
    expect(screen.getByRole('option', { name: /Watcher/ })).toHaveAccessibleName(/영상/)
    expect(screen.getByRole('option', { name: /Blind/ })).not.toHaveAccessibleName(/영상/)
  })
})

describe('StageModelSelect with a workflow availability verdict (T112)', () => {
  const VIDEO: FakeProvidersOptions['models'] = [
    { providerId: 'p', modelId: 'google', label: 'Gemini', vision: true, videoInput: true },
    { providerId: 'p', modelId: 'qwen', label: 'Qwen', vision: true, videoInput: true },
    { providerId: 'p', modelId: 'amazon', label: 'Nova', vision: true, videoInput: true },
    {
      providerId: 'p',
      modelId: 'nokey',
      label: 'Keyless',
      vision: true,
      videoInput: true,
      disabledReason: 'API key not configured',
    },
  ]
  const ready = (refused: Record<string, string>): ModelAvailability => ({
    kind: 'ready',
    resolve: (ref: ModelRef): ModelVerdict =>
      ref.modelId in refused
        ? { usable: false, reason: refused[ref.modelId] }
        : ref.modelId === 'qwen' || ref.modelId === 'nokey'
          ? { usable: true }
          : { usable: false, reason: '' },
  })

  function renderWith(availability: ModelAvailability, options: FakeProvidersOptions = {}) {
    const transport = createFakeProviderTransport({ models: VIDEO, ...options })
    const queryClient = createTestQueryClient()
    render(<StageModelSelect stage="observe" availability={availability} />, {
      wrapper: withProviders(transport, queryClient),
    })
  }

  it('greys refused models with their reason, keeps a refused saved choice selected, and saves no refused pick', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderWith(ready({ google: '요금 상한을 확인할 수 없는 경로예요' }), {
      calls,
      selections: [{ stage: Stage.OBSERVE, providerId: 'p', modelId: 'google' }],
    })
    const trigger = await openPanel(user, /관찰 모델/)
    expect(trigger).toHaveTextContent('Gemini')
    expect(trigger).toHaveAccessibleDescription(/요금 상한을 확인할 수 없는 경로예요/)
    const gemini = screen.getByRole('option', { name: /Gemini/ })
    expect(gemini).toHaveAttribute('aria-disabled', 'true')
    expect(gemini).toHaveTextContent('요금 상한을 확인할 수 없는 경로예요')
    // Unresolved is unusable too, with the generic note rather than an invented reason.
    const nova = screen.getByRole('option', { name: /Nova/ })
    expect(nova).toHaveAttribute('aria-disabled', 'true')
    // The provider's own state keeps precedence over the workflow verdict.
    expect(screen.getByRole('option', { name: /Keyless/ })).toHaveTextContent(
      'API key not configured',
    )
    expect(screen.getByRole('option', { name: /Qwen/ })).not.toHaveAttribute('aria-disabled')
    await user.click(nova)
    expect(calls).not.toContain('SaveSelection')
    expect(trigger).toHaveTextContent('Gemini')
  })

  it('lets the user pick a usable model and only that', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderWith(ready({}), { calls })
    await openPanel(user, /관찰 모델/)
    await user.click(screen.getByRole('option', { name: /Qwen/ }))
    await waitFor(() => expect(calls).toContain('SaveSelection'))
  })

  it('closes every pick while the verdict is loading or failed and offers a retry', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    const { unmount } = render(<></>)
    unmount()
    renderWith({ kind: 'loading' }, { calls })
    let trigger = await openPanel(user, /관찰 모델/)
    expect(trigger).toHaveAccessibleDescription(/쓸 수 있는 모델인지 확인하는 중이에요/)
    for (const name of [/Gemini/, /Qwen/])
      expect(screen.getByRole('option', { name })).toHaveAttribute('aria-disabled', 'true')
    await user.click(screen.getByRole('option', { name: /Qwen/ }))
    expect(calls).not.toContain('SaveSelection')
    let retried = 0
    const failed = () => {
      cleanup()
      renderWith({ kind: 'failed', retry: () => retried++ }, { calls })
    }
    failed()
    trigger = await openPanel(user, /관찰 모델/)
    expect(trigger).toHaveAccessibleDescription(/확인하지 못했어요/)
    await user.keyboard('{Escape}')
    await user.click(screen.getByRole('button', { name: '다시 확인' }))
    expect(retried).toBe(1)
    expect(calls).not.toContain('SaveSelection')
  })
})
