import { afterEach, describe, expect, it } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { Stage } from '@/shared/api'
import { createTestQueryClient, withProviders } from '@/test/session'
import { type FakeProvidersOptions, createFakeProviderTransport } from '@/test/providers'
import { StageModelSelect } from './StageModelSelect'

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
    renderSelect('write', { calls })

    const select = await screen.findByRole('combobox', { name: '작성 모델' })
    await waitFor(() => expect(select).toBeEnabled())
    expect(select).toHaveValue('')
    expect(screen.getByRole('option', { name: '모델을 선택하세요' })).toBeInTheDocument()
    expect(calls).not.toContain('SaveSelection')
  })

  // AC3: observe lists vision models only, with badges.
  it('lists only vision models for observe and badges what each can do', async () => {
    renderSelect('observe')

    await screen.findByRole('option', { name: 'Free 👁' })
    expect(screen.queryByRole('option', { name: /Writer/ })).not.toBeInTheDocument()
    expect(
      screen.getByRole('option', { name: 'Claude 👁 구조화 응답 — API key not configured' }),
    ).toBeDisabled()
  })

  // AC2: a provider without a key is greyed with the exact reason and cannot be picked.
  it('greys a model whose provider has no key, with the reason', async () => {
    renderSelect('write')

    const option = await screen.findByRole('option', { name: /Claude/ })
    expect(option).toBeDisabled()
    expect(option).toHaveTextContent('API key not configured')
  })

  // AC4 (client half): picking saves, and the choice is shown at once.
  it('saves a pick and shows it as selected', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderSelect('write', { calls })

    const select = await screen.findByRole('combobox', { name: '작성 모델' })
    await waitFor(() => expect(select).toBeEnabled())
    await user.selectOptions(select, 'openrouter/writer')

    await waitFor(() => expect(select).toHaveValue('openrouter/writer'))
    expect(calls).toContain('SaveSelection')
  })

  it('restores a saved choice', async () => {
    renderSelect('write', {
      selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
    })

    const select = await screen.findByRole('combobox', { name: '작성 모델' })
    await waitFor(() => expect(select).toHaveValue('openrouter/writer'))
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
      await user.selectOptions(select, 'openrouter/writer')

      expect(await screen.findByRole('alert')).toHaveTextContent(message)
      expect(select).toHaveAttribute('aria-invalid', 'true')
      expect(select).toHaveAccessibleDescription(message)
      expect(document.body).not.toHaveTextContent('private backend prose')
      expect(document.body).not.toHaveTextContent('[unavailable]')
    },
  )

  // AC5: a vanished model is shown greyed with the reason and counts as unselected.
  it('shows a vanished saved model greyed with the reason and treats the stage as unselected', async () => {
    renderSelect('write', {
      selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'gone' }],
    })

    const gone = await screen.findByRole('option', { name: /openrouter\/gone/ })
    expect(gone).toBeDisabled()
    // The option text is the choice, not the explanation: this entry is the closed control's own
    // value, so the reason is rendered under the field where it cannot be ellipsised away.
    expect(screen.getByRole('combobox', { name: '작성 모델' })).toHaveAccessibleDescription(
      '등록된 모델 목록에서 사라졌어요',
    )
    // The select points at the greyed entry, not at any usable model.
    expect(screen.getByRole('combobox', { name: '작성 모델' })).toHaveValue('__unavailable__')
  })

  it('says so when the catalog cannot be loaded, without calling a saved model vanished', async () => {
    renderSelect('write', {
      listFails: true,
      selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
    })

    expect(await screen.findByRole('alert')).toHaveTextContent('모델 목록을 불러오지 못했어요')
    expect(screen.getByRole('combobox', { name: '작성 모델' })).toHaveAttribute(
      'aria-invalid',
      'true',
    )
    expect(screen.getByRole('combobox', { name: '작성 모델' })).toHaveAccessibleDescription(
      '모델 목록을 불러오지 못했어요.',
    )
    expect(screen.queryByRole('option', { name: /사라졌어요/ })).not.toBeInTheDocument()
  })

  // A model saved for observe that the yaml no longer marks as vision is unusable there.
  it('greys a saved model the stage can no longer use', async () => {
    renderSelect('observe', {
      selections: [{ stage: Stage.OBSERVE, providerId: 'openrouter', modelId: 'writer' }],
    })

    const option = await screen.findByRole('option', { name: /openrouter\/writer/ })
    expect(option).toBeDisabled()
    expect(screen.getByRole('combobox', { name: '관찰 모델' })).toHaveAccessibleDescription(
      '이 단계에서는 쓸 수 없는 모델이에요',
    )
    expect(screen.getByRole('combobox', { name: '관찰 모델' })).toHaveValue('__unavailable__')
  })
})
