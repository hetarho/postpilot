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
  expect(screen.getByRole('dialog', { name: '테스트 옵션' })).toHaveClass('bottom-full')
  expect(screen.getByLabelText('옵션 값')).toHaveFocus()
  await user.keyboard('{Escape}')
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(trigger).toHaveFocus()
  expect(trigger).toHaveClass('min-h-11')
})

it('bounds a below-header panel to the viewport and lets its contents scroll', async () => {
  const user = userEvent.setup()
  render(
    <Popover label="계정" placement="below">
      {() => <button>로그아웃</button>}
    </Popover>,
  )

  await user.click(screen.getByRole('button', { name: '계정' }))
  expect(screen.getByRole('dialog', { name: '계정' })).toHaveClass(
    'max-h-popover-below-max',
    'overflow-y-auto',
    'overscroll-contain',
  )
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

it('cycles Tab focus inside the open dialog', async () => {
  const user = userEvent.setup()
  render(
    <div>
      <Popover label="테스트 옵션">
        {() => (
          <>
            <input aria-label="첫 번째 옵션" />
            <button>마지막 옵션</button>
          </>
        )}
      </Popover>
      <button>바깥</button>
    </div>,
  )

  await user.click(screen.getByRole('button', { name: '테스트 옵션' }))
  const first = screen.getByRole('textbox', { name: '첫 번째 옵션' })
  const last = screen.getByRole('button', { name: '마지막 옵션' })
  expect(first).toHaveFocus()

  await user.tab({ shift: true })
  expect(last).toHaveFocus()
  await user.tab()
  expect(first).toHaveFocus()
  expect(screen.getByRole('button', { name: '바깥' })).not.toHaveFocus()
})

it('includes a native details summary in the focus cycle', async () => {
  const user = userEvent.setup()
  render(
    <Popover label="오류 세부정보">
      {() => (
        <>
          <button>다시 시도</button>
          <details>
            <summary>기술 세부 정보</summary>
            실패 코드
          </details>
        </>
      )}
    </Popover>,
  )

  await user.click(screen.getByRole('button', { name: '오류 세부정보' }))
  expect(screen.getByRole('button', { name: '다시 시도' })).toHaveFocus()
  await user.tab()
  expect(screen.getByText('기술 세부 정보')).toHaveFocus()
  await user.tab()
  expect(screen.getByRole('button', { name: '다시 시도' })).toHaveFocus()
})
