import { afterEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Stage } from '@/shared/api'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import type { FakeVoiceOptions } from '@/test/voice'
import { emptyStructuredVoiceProfile, type VoiceProfile } from '@/entities/voice-profile'
import { LearnVoiceForm } from './LearnVoiceForm'

const EMPTY_PROFILE: VoiceProfile = {
  styleguide: '',
  rules: '',
  updatedAt: '',
  samples: [],
  activeJobId: '',
  legacyManualGuidance: '',
  structured: emptyStructuredVoiceProfile(),
  finalizedSourceCount: 0,
  canValidate: false,
}

function renderForm({
  profile = EMPTY_PROFILE,
  voice = {},
}: { profile?: VoiceProfile; voice?: FakeVoiceOptions } = {}) {
  const calls: string[] = []
  const transport = createFakeAuthTransport({
    calls,
    providers: {
      models: [{ providerId: 'openrouter', modelId: 'writer' }],
      selections: [{ stage: Stage.ANALYZE, providerId: 'openrouter', modelId: 'writer' }],
    },
    voice,
  })
  const onStarted = vi.fn()
  render(<LearnVoiceForm ownerId="alice" profile={profile} onStarted={onStarted} />, {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  return { calls, onStarted }
}

afterEach(() => vi.restoreAllMocks())

describe('LearnVoiceForm', () => {
  it('stays disabled at 199 characters and enables at 200 with a selected model', async () => {
    renderForm()
    const body = screen.getByLabelText('내가 쓴 글')
    const submit = screen.getByRole('button', { name: '학습' })

    fireEvent.change(body, { target: { value: '가'.repeat(199) } })
    expect(screen.getByText('199 / 200자')).toBeInTheDocument()
    expect(submit).toBeDisabled()

    fireEvent.change(body, { target: { value: '가'.repeat(200) } })
    await waitFor(() => expect(submit).toBeEnabled())
  })

  it('requires an analyze-stage model', async () => {
    const transport = createFakeAuthTransport({
      providers: { models: [{ providerId: 'openrouter', modelId: 'writer' }] },
    })
    render(<LearnVoiceForm ownerId="alice" profile={EMPTY_PROFILE} onStarted={vi.fn()} />, {
      wrapper: withProviders(transport, createTestQueryClient()),
    })

    fireEvent.change(screen.getByLabelText('내가 쓴 글'), {
      target: { value: '가'.repeat(200) },
    })
    expect(await screen.findByText('모델을 선택하세요')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '학습' })).toBeDisabled()
  })

  it('asks before replacing an existing styleguide', async () => {
    const { calls } = renderForm({ profile: { ...EMPTY_PROFILE, styleguide: '# 종결어미' } })
    fireEvent.change(screen.getByLabelText('내가 쓴 글'), {
      target: { value: '가'.repeat(200) },
    })
    await waitFor(() => expect(screen.getByRole('button', { name: '학습' })).toBeEnabled())
    await userEvent.click(screen.getByRole('button', { name: '학습' }))

    // The overwrite warning is the Dialog sheet, not window.confirm, which a mobile browser lets
    // the user suppress for the rest of the session.
    expect(await screen.findByRole('dialog')).toHaveTextContent(
      '재분석하면 현재 문체 규칙을 덮어씁니다',
    )
    await userEvent.click(screen.getByRole('button', { name: '취소' }))
    expect(calls).not.toContain('AddVoiceSample')
  })

  it('shows the server InvalidArgument message verbatim', async () => {
    const message = 'sample has 199 characters; at least 200 are required'
    renderForm({ voice: { addError: message } })
    fireEvent.change(screen.getByLabelText('내가 쓴 글'), {
      target: { value: '가'.repeat(200) },
    })
    await waitFor(() => expect(screen.getByRole('button', { name: '학습' })).toBeEnabled())
    await userEvent.click(screen.getByRole('button', { name: '학습' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(message)
  })

  it('keeps a new sample typed while the previous submission is pending', async () => {
    let release!: () => void
    const addGate = new Promise<void>((resolve) => {
      release = resolve
    })
    const { calls, onStarted } = renderForm({ voice: { addGate } })
    const body = screen.getByLabelText('내가 쓴 글')
    fireEvent.change(body, { target: { value: '가'.repeat(200) } })
    await waitFor(() => expect(screen.getByRole('button', { name: '학습' })).toBeEnabled())
    await userEvent.click(screen.getByRole('button', { name: '학습' }))
    await waitFor(() => expect(calls).toContain('AddVoiceSample'))

    fireEvent.change(body, { target: { value: '다음 샘플' } })
    release()

    await waitFor(() => expect(onStarted).toHaveBeenCalled())
    expect(body).toHaveValue('다음 샘플')
  })
})
