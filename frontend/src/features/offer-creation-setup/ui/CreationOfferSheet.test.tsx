import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import type { CreationOfferKind, CreationOfferMissing } from '../model/offer'
import { CreationOfferSheet } from './CreationOfferSheet'

function renderSheet(kind: CreationOfferKind, missing: CreationOfferMissing) {
  const handlers = {
    onCreateVoice: vi.fn(),
    onContinueVoice: vi.fn(),
    onCreateTemplate: vi.fn(),
    onClose: vi.fn(),
    onDismiss: vi.fn(),
  }
  render(<CreationOfferSheet kind={kind} open missing={missing} {...handlers} />)
  return handlers
}

beforeEach(() => {
  initializeI18n('ko')
})
afterEach(() => {
  cleanup()
})

describe('CreationOfferSheet', () => {
  it('titles a post missing both and offers each kind', async () => {
    const user = userEvent.setup()
    const handlers = renderSheet('post', { voice: { action: 'create' }, template: true })

    const dialog = await screen.findByRole('dialog', { name: '말투와 템플릿이 아직 없어요' })
    await user.click(within(dialog).getByRole('button', { name: '새 말투 만들기' }))
    await user.click(within(dialog).getByRole('button', { name: '새 템플릿 만들기' }))

    expect(handlers.onCreateVoice).toHaveBeenCalledOnce()
    expect(handlers.onCreateTemplate).toHaveBeenCalledOnce()
  })

  it('continues the voice being made by its id', async () => {
    const user = userEvent.setup()
    const handlers = renderSheet('post', {
      voice: { action: 'continue', voiceId: 'voice-half' },
      template: false,
    })

    const dialog = await screen.findByRole('dialog', { name: '말투가 아직 없어요' })
    expect(within(dialog).queryByRole('button', { name: '새 템플릿 만들기' })).not.toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: '말투 이어서 만들기' }))

    expect(handlers.onContinueVoice).toHaveBeenCalledWith('voice-half')
    expect(handlers.onCreateVoice).not.toHaveBeenCalled()
  })

  it('titles a post missing only its template', async () => {
    renderSheet('post', { template: true })
    expect(await screen.findByRole('dialog', { name: '템플릿이 아직 없어요' })).toBeInTheDocument()
  })

  it('offers a clip only its video template, with its own close wording', async () => {
    const user = userEvent.setup()
    const handlers = renderSheet('clip', { template: true })

    const dialog = await screen.findByRole('dialog', { name: '영상 템플릿이 아직 없어요' })
    expect(within(dialog).queryByRole('button', { name: /말투/ })).not.toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: '새 영상 템플릿 만들기' }))
    await user.click(within(dialog).getByRole('button', { name: '이대로 만들기' }))

    expect(handlers.onCreateTemplate).toHaveBeenCalledOnce()
    expect(handlers.onClose).toHaveBeenCalledOnce()
  })

  it('closes on Escape and dismisses on 다시 묻지 않기', async () => {
    const user = userEvent.setup()
    const handlers = renderSheet('post', { template: true })

    const dialog = await screen.findByRole('dialog', { name: '템플릿이 아직 없어요' })
    await user.click(within(dialog).getByRole('button', { name: '다시 묻지 않기' }))
    await user.keyboard('{Escape}')

    expect(handlers.onDismiss).toHaveBeenCalledOnce()
    expect(handlers.onClose).toHaveBeenCalledOnce()
  })
})
