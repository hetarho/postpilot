import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import type { SpeechProfileChoice } from '../model/speech'
import { SpeechProfilePicker } from './SpeechProfilePicker'
import { i18n } from '../config/i18n'

const profile: SpeechProfileChoice = {
  id: 'voice-profile',
  revision: 2n,
  label: 'Korean voice',
  designModel: { providerId: 'speech', modelId: 'design' },
  designLabel: 'Design model',
  speechModel: { providerId: 'speech', modelId: 'synth' },
  speechLabel: 'Synthesis model',
  grade: 'value',
  requiredPlan: 'light',
  entitled: true,
  available: true,
  unavailableReason: '',
  descriptionMax: 1000,
  previewMin: 100,
  previewMax: 1000,
  speechMax: 1000,
  voiceReady: true,
  exportReady: false,
}

describe('explicit speech model choices', () => {
  it('starts without a selection and selects the exact profile revision explicitly', async () => {
    const select = vi.fn()
    const user = userEvent.setup()
    render(<SpeechProfilePicker profiles={[profile]} selectedId={null} onSelect={select} />)
    expect(screen.getByRole('combobox', { name: /목소리 생성 모델/ })).toHaveTextContent(
      '모델을 선택하세요',
    )
    expect(select).not.toHaveBeenCalled()
    await user.click(screen.getByRole('combobox', { name: /목소리 생성 모델/ }))
    await user.click(screen.getByRole('option', { name: /Design model → Synthesis model/ }))
    expect(select).toHaveBeenCalledWith(profile)
  })

  it('keeps unavailable and plan-locked models visible with a reason and refuses selection', async () => {
    const select = vi.fn()
    const user = userEvent.setup()
    const missing = {
      ...profile,
      id: 'missing',
      available: false,
      unavailableReason: 'SPEECH_CONNECTION_UNAVAILABLE',
    }
    const locked = {
      ...profile,
      id: 'locked',
      grade: 'top' as const,
      requiredPlan: 'max',
      entitled: false,
      available: false,
      unavailableReason: 'MODEL_PLAN_REQUIRED',
    }
    render(<SpeechProfilePicker profiles={[missing, locked]} selectedId={null} onSelect={select} />)
    await user.click(screen.getByRole('combobox', { name: /목소리 생성 모델/ }))
    const unavailable = screen.getByRole('option', {
      name: /목소리 생성 서비스를 지금 사용할 수 없어요/,
    })
    const requiresPlan = screen.getByRole('option', { name: /max 요금제부터/ })
    expect(unavailable).toHaveAttribute('aria-disabled', 'true')
    expect(requiresPlan).toHaveAttribute('aria-disabled', 'true')
    await user.click(unavailable)
    await user.click(requiresPlan)
    expect(select).not.toHaveBeenCalled()
  })

  it('shows associated synthesis and limits in English without cost data', () => {
    initializeI18n('en')
    try {
      render(
        <SpeechProfilePicker profiles={[profile]} selectedId={profile.id} onSelect={vi.fn()} />,
      )
      expect(screen.getByRole('combobox', { name: /Voice creation model/ })).toHaveTextContent(
        'Synthesis model',
      )
      expect(screen.getByRole('status')).toHaveTextContent('1000')
      expect(document.body.textContent).not.toMatch(/USD|supplier|credits per/)
    } finally {
      initializeI18n('ko')
    }
  })
  it('keeps every speech reason and customer label in both languages', () => {
    expect(Object.keys(i18n.ko.speech)).toEqual(Object.keys(i18n.en.speech))
    expect(Object.keys(i18n.ko.speech.reason)).toEqual(Object.keys(i18n.en.speech.reason))
  })
})
