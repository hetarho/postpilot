import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { StyleguideEditor } from './StyleguideEditor'

describe('StyleguideEditor', () => {
  it('keeps edits made while an earlier save is pending', async () => {
    let release!: () => void
    const updateGate = new Promise<void>((resolve) => {
      release = resolve
    })
    const calls: string[] = []
    const transport = createFakeAuthTransport({ calls, voice: { updateGate } })
    render(<StyleguideEditor ownerId="alice" voiceId="voice-default" styleguide="기존" />, {
      wrapper: withProviders(transport, createTestQueryClient()),
    })
    const editor = screen.getByLabelText('문체 규칙')
    fireEvent.change(editor, { target: { value: '첫 저장' } })
    await userEvent.click(screen.getByRole('button', { name: '저장' }))
    await waitFor(() => expect(calls).toContain('UpdateVoiceProfile'))

    fireEvent.change(editor, { target: { value: '응답 전에 이어 쓴 내용' } })
    release()

    await waitFor(() => expect(screen.getByRole('button', { name: '저장' })).toBeEnabled())
    expect(editor).toHaveValue('응답 전에 이어 쓴 내용')
  })
})
