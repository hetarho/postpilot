import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { SM_MEDIA_QUERY } from '../media-query/useMediaQuery'
import { ActionMenu } from './ActionMenu'

function items(onCompare = vi.fn(), onOther = vi.fn()) {
  return [
    {
      id: 'compare',
      label: 'A/B 비교',
      description: '두 모델로 써 보고 고릅니다',
      onSelect: onCompare,
    },
    {
      id: 'other',
      label: '다른 방법',
      disabledReason: '작성 모델을 먼저 고르세요',
      onSelect: onOther,
    },
  ]
}

describe('ActionMenu from sm: up', () => {
  const original = window.matchMedia
  beforeEach(() => {
    window.matchMedia = ((query: string) => ({
      matches: query === SM_MEDIA_QUERY,
      media: query,
      onchange: null,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      addListener: () => undefined,
      removeListener: () => undefined,
      dispatchEvent: () => false,
    })) as typeof window.matchMedia
  })
  afterEach(() => {
    window.matchMedia = original
  })

  it('opens a menu of actions on the first row and runs the one chosen', async () => {
    const user = userEvent.setup()
    const onCompare = vi.fn()
    render(<ActionMenu label="다른 방법으로 쓰기" items={items(onCompare)} />)

    const trigger = screen.getByRole('button', { name: '다른 방법으로 쓰기' })
    expect(trigger).toHaveAttribute('aria-haspopup', 'menu')
    await user.click(trigger)
    const menu = screen.getByRole('menu', { name: '다른 방법으로 쓰기' })
    const rows = screen.getAllByRole('menuitem')
    expect(rows).toHaveLength(2)
    await waitFor(() => expect(rows[0]).toHaveFocus())
    expect(menu).toContainElement(screen.getByText('두 모델로 써 보고 고릅니다'))

    await user.keyboard('{Enter}')
    expect(onCompare).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
    await waitFor(() => expect(trigger).toHaveFocus())
  })

  it('keeps a disabled action with its reason and runs nothing', async () => {
    const user = userEvent.setup()
    const onOther = vi.fn()
    render(<ActionMenu label="다른 방법으로 쓰기" items={items(vi.fn(), onOther)} />)
    await user.click(screen.getByRole('button', { name: '다른 방법으로 쓰기' }))
    const blocked = screen.getByRole('menuitem', { name: '다른 방법' })
    expect(blocked).toHaveAttribute('aria-disabled', 'true')
    expect(blocked).toHaveAccessibleDescription('작성 모델을 먼저 고르세요')
    await user.click(blocked)
    expect(onOther).not.toHaveBeenCalled()
    expect(screen.getByRole('menu')).toBeInTheDocument()
  })

  it('moves with the arrows, wraps, and closes on Escape back to the trigger', async () => {
    const user = userEvent.setup()
    render(<ActionMenu label="다른 방법으로 쓰기" items={items()} />)
    const trigger = screen.getByRole('button', { name: '다른 방법으로 쓰기' })
    trigger.focus()
    await user.keyboard('{ArrowDown}')
    const rows = screen.getAllByRole('menuitem')
    await waitFor(() => expect(rows[0]).toHaveFocus())
    await user.keyboard('{ArrowDown}')
    expect(rows[1]).toHaveFocus()
    await user.keyboard('{ArrowDown}')
    expect(rows[0]).toHaveFocus()
    await user.keyboard('{ArrowUp}')
    expect(rows[1]).toHaveFocus()
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
    await waitFor(() => expect(trigger).toHaveFocus())
  })

  it('closes on a press outside', async () => {
    const user = userEvent.setup()
    render(
      <>
        <ActionMenu label="다른 방법으로 쓰기" items={items()} />
        <button type="button">밖</button>
      </>,
    )
    await user.click(screen.getByRole('button', { name: '다른 방법으로 쓰기' }))
    await user.click(screen.getByRole('button', { name: '밖' }))
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  })
})

describe('ActionMenu below sm:', () => {
  it('opens as a bottom sheet with a visible way out', async () => {
    const user = userEvent.setup()
    const onCompare = vi.fn()
    render(<ActionMenu label="다른 방법으로 쓰기" items={items(onCompare)} />)
    await user.click(screen.getByRole('button', { name: '다른 방법으로 쓰기' }))
    const sheet = screen.getByRole('dialog', { name: '다른 방법으로 쓰기' })
    expect(sheet).toHaveClass('rounded-t-xl')
    expect(screen.getByRole('menu', { name: '다른 방법으로 쓰기' })).toBeInTheDocument()
    await user.click(screen.getByRole('menuitem', { name: /A\/B 비교/ }))
    expect(onCompare).toHaveBeenCalledTimes(1)

    await user.click(screen.getByRole('button', { name: '다른 방법으로 쓰기' }))
    await user.click(screen.getByRole('button', { name: '닫기' }))
    await waitFor(() => expect(screen.queryByRole('menu')).not.toBeInTheDocument())
  })
})
