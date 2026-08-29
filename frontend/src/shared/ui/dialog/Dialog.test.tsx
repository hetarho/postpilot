import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import { Dialog } from './Dialog'

it('is modal, closes on escape, and restores focus', async () => {
  const close = vi.fn()
  const trigger = document.createElement('button')
  document.body.append(trigger)
  trigger.focus()
  const { rerender } = render(
    <Dialog open title="덮어쓰기" confirmLabel="적용" onConfirm={vi.fn()} onClose={close}>
      설명
    </Dialog>,
  )
  expect(screen.getByRole('dialog')).toHaveFocus()
  await userEvent.setup().keyboard('{Escape}')
  expect(close).toHaveBeenCalledOnce()
  rerender(
    <Dialog open={false} title="덮어쓰기" confirmLabel="적용" onConfirm={vi.fn()} onClose={close}>
      설명
    </Dialog>,
  )
  expect(trigger).toHaveFocus()
  trigger.remove()
})
