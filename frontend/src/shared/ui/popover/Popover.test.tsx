import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it } from 'vitest'
import { Popover } from './Popover'

it('opens from the keyboard, closes on Escape, and restores trigger focus', async () => {
  const user = userEvent.setup()
  render(<Popover label="테스트 옵션">{() => <input aria-label="옵션 값" />}</Popover>)
  const trigger = screen.getByRole('button', { name: '테스트 옵션' })
  trigger.focus()
  await user.keyboard('{Enter}')
  expect(screen.getByRole('dialog', { name: '테스트 옵션' })).toBeInTheDocument()
  expect(screen.getByLabelText('옵션 값')).toHaveFocus()
  await user.keyboard('{Escape}')
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(trigger).toHaveFocus()
  expect(trigger).toHaveClass('min-h-11')
})

it('dismisses when the user presses outside', async () => {
  const user = userEvent.setup()
  render(
    <div>
      <Popover label="테스트 옵션">{() => <span>내용</span>}</Popover>
      <button>바깥</button>
    </div>,
  )
  await user.click(screen.getByRole('button', { name: '테스트 옵션' }))
  await user.click(screen.getByRole('button', { name: '바깥' }))
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})
