import { describe, expect, it } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { VoiceValue } from '@/entities/voice-profile'
import { VoiceLayer } from '@/shared/api'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import type { FakeVoiceOptions } from '@/test/voice'
import { ProfileField } from './ProfileField'

const ANALYZED: VoiceValue = { value: '담백한 어휘', source: 'analyzed', unknown: false }

function renderField(value: VoiceValue = ANALYZED, voice: FakeVoiceOptions = {}) {
  const calls: string[] = []
  const transport = createFakeAuthTransport({ calls, voice })
  render(
    <ProfileField
      ownerId="alice"
      label="어휘 성격"
      layer={VoiceLayer.LEXICAL}
      field="description"
      value={value}
    />,
    { wrapper: withProviders(transport, createTestQueryClient()) },
  )
  return { calls }
}

describe('ProfileField', () => {
  // Change 04 A6.
  it('reads as text until its edit control is pressed', async () => {
    renderField()

    expect(screen.getByText('담백한 어휘')).toBeInTheDocument()
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()

    await userEvent.setup().click(screen.getByRole('button', { name: '어휘 성격 수정' }))

    expect(screen.getByRole('textbox')).toHaveValue('담백한 어휘')
  })

  // Change 04 A7, second half.
  it('discards the draft and restores the published value on 취소', async () => {
    const user = userEvent.setup()
    renderField()

    await user.click(screen.getByRole('button', { name: '어휘 성격 수정' }))
    await user.type(screen.getByRole('textbox'), ' 그리고 짧게')
    await user.click(screen.getByRole('button', { name: '취소' }))

    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
    expect(screen.getByText('담백한 어휘')).toBeInTheDocument()

    // Reopening starts from the published value, not from the abandoned draft.
    await user.click(screen.getByRole('button', { name: '어휘 성격 수정' }))
    expect(screen.getByRole('textbox')).toHaveValue('담백한 어휘')
  })

  // Change 04 A7, third half: a rejected save must never cost the owner their text.
  it('stays in edit mode with the draft intact when the save is rejected', async () => {
    const user = userEvent.setup()
    const { calls } = renderField(ANALYZED, { overrideFails: true })

    await user.click(screen.getByRole('button', { name: '어휘 성격 수정' }))
    await user.clear(screen.getByRole('textbox'))
    await user.type(screen.getByRole('textbox'), '새 설명')
    await user.click(screen.getByRole('button', { name: '저장' }))

    await waitFor(() => expect(calls).toContain('UpdateVoiceOverride'))
    expect(await screen.findByRole('alert')).toHaveTextContent('직접 설정을 저장하지 못했어요')
    expect(screen.getByRole('textbox')).toHaveValue('새 설명')
  })

  // Change 04 A8.
  it('offers 직접 설정 해제 only for a manually set field', async () => {
    const user = userEvent.setup()
    renderField()
    await user.click(screen.getByRole('button', { name: '어휘 성격 수정' }))
    expect(screen.queryByRole('button', { name: '직접 설정 해제' })).not.toBeInTheDocument()
  })

  it('clears a manual override back to the analyzed value', async () => {
    const user = userEvent.setup()
    const { calls } = renderField({ value: '내가 쓴 설명', source: 'manual', unknown: false })

    await user.click(screen.getByRole('button', { name: '어휘 성격 수정' }))
    await user.click(screen.getByRole('button', { name: '직접 설정 해제' }))

    await waitFor(() => expect(calls).toContain('UpdateVoiceOverride'))
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
  })
})
