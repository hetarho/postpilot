import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { Sheet } from './Sheet'

afterEach(cleanup)

describe('Sheet', () => {
  it('portals a bottom sheet whose body is the one scroller, and locks the page scroll', () => {
    const { unmount } = render(
      <Sheet open label="옵션" footer={<button>닫기</button>} onClose={() => undefined}>
        <p>내용</p>
      </Sheet>,
    )

    const panel = screen.getByRole('dialog', { name: '옵션' })
    expect(panel.parentElement?.parentElement).toBe(document.body)
    expect(panel).toHaveClass('rounded-t-xl', 'pb-sheet-b', 'max-h-sheet', 'md:rounded-xl')
    expect(screen.getByText('내용').parentElement).toHaveClass(
      'overflow-y-auto',
      'overscroll-contain',
    )
    expect(document.body.style.overflow).toBe('hidden')

    unmount()
    expect(document.body.style.overflow).toBe('')
  })

  it('takes its name from a visible heading when one is given', () => {
    render(
      <Sheet
        open
        labelledBy="sheet-heading"
        header={<h2 id="sheet-heading">생성 옵션</h2>}
        onClose={() => undefined}
      >
        내용
      </Sheet>,
    )
    expect(screen.getByRole('dialog', { name: '생성 옵션' })).toBeInTheDocument()
  })

  it('closes on Escape and on a scrim press, and returns focus where it found it', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    function Harness({ open }: { open: boolean }) {
      return (
        <>
          <button>여는 버튼</button>
          <Sheet open={open} label="옵션" onClose={onClose}>
            <button>안쪽</button>
          </Sheet>
        </>
      )
    }
    const { rerender } = render(<Harness open={false} />)
    const opener = screen.getByRole('button', { name: '여는 버튼' })
    opener.focus()

    rerender(<Harness open />)
    await user.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalledTimes(1)

    await user.click(screen.getByRole('dialog', { name: '옵션' }).parentElement!)
    expect(onClose).toHaveBeenCalledTimes(2)

    rerender(<Harness open={false} />)
    expect(opener).toHaveFocus()
  })

  it('traps Tab inside the panel', async () => {
    const user = userEvent.setup()
    render(
      <>
        <button>바깥</button>
        <Sheet open label="옵션" footer={<button>두 번째</button>} onClose={() => undefined}>
          <button>첫 번째</button>
        </Sheet>
      </>,
    )

    await user.tab()
    expect(screen.getByRole('button', { name: '첫 번째' })).toHaveFocus()
    await user.tab()
    expect(screen.getByRole('button', { name: '두 번째' })).toHaveFocus()
    await user.tab()
    expect(screen.getByRole('button', { name: '첫 번째' })).toHaveFocus()
  })
})

describe('rising and sinking', () => {
  it('rides in from the bottom edge on a phone and settles in place from md: up', () => {
    render(
      <Sheet open label="옵션" onClose={() => undefined}>
        내용
      </Sheet>,
    )
    const panel = screen.getByRole('dialog', { name: '옵션' })
    expect(panel).toHaveClass('animate-sheet-in', 'md:animate-dialog-in')
    expect(panel.parentElement).toHaveClass('animate-scrim-in')
  })

  // The departure is the half React does not give away for free: the node would be gone before a
  // single frame of it played, so the panel outlives `open` by exactly one animation.
  it('sinks back down before it unmounts, having already given focus back', () => {
    const onClose = vi.fn()
    function Harness({ open }: { open: boolean }) {
      return (
        <>
          <button>여는 버튼</button>
          <Sheet open={open} label="옵션" onClose={onClose}>
            <button>안쪽</button>
          </Sheet>
        </>
      )
    }
    const { rerender } = render(<Harness open={false} />)
    const opener = screen.getByRole('button', { name: '여는 버튼' })
    opener.focus()

    rerender(<Harness open />)
    // What tells the sheet this environment animates at all. jsdom never fires it on its own.
    fireEvent.animationStart(screen.getByRole('dialog', { name: '옵션' }))

    rerender(<Harness open={false} />)
    const leaving = screen.getByRole('dialog', { name: '옵션' })
    expect(leaving).toHaveClass('animate-sheet-out')
    expect(leaving).toHaveAttribute('inert')
    expect(leaving.parentElement).toHaveClass('animate-scrim-out', 'pointer-events-none')
    // Everything the overlay OWED the page is already back, so what is left is only a picture.
    expect(document.body.style.overflow).toBe('')
    expect(opener).toHaveFocus()

    fireEvent.animationEnd(leaving)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  // Nothing animated it in, so nothing will animate it out: waiting on `animationend` here would
  // leave the panel mounted behind a scrim forever.
  it('goes at once where nothing animates', () => {
    function Harness({ open }: { open: boolean }) {
      return (
        <Sheet open={open} label="옵션" onClose={() => undefined}>
          내용
        </Sheet>
      )
    }
    const { rerender } = render(<Harness open />)
    expect(screen.getByRole('dialog', { name: '옵션' })).toBeInTheDocument()
    rerender(<Harness open={false} />)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})

describe('stacked sheets', () => {
  // The real shape: a confirmation opened from a control INSIDE an already-open sheet. Escape
  // dismisses the confirmation, not the surface it was opened from.
  it('lets one Escape dismiss only the one opened last', async () => {
    const user = userEvent.setup()
    const closeOuter = vi.fn()
    const closeInner = vi.fn()
    function Harness({ innerOpen }: { innerOpen: boolean }) {
      return (
        <Sheet open label="바깥" onClose={closeOuter}>
          <Sheet open={innerOpen} label="안쪽" onClose={closeInner}>
            내용
          </Sheet>
        </Sheet>
      )
    }
    const { rerender } = render(<Harness innerOpen={false} />)
    rerender(<Harness innerOpen />)

    await user.keyboard('{Escape}')
    expect(closeInner).toHaveBeenCalledTimes(1)
    expect(closeOuter).not.toHaveBeenCalled()

    rerender(<Harness innerOpen={false} />)
    await user.keyboard('{Escape}')
    expect(closeOuter).toHaveBeenCalledTimes(1)
  })

  it('keeps Tab off controls it cannot reach', async () => {
    const user = userEvent.setup()
    render(
      <>
        <button>바깥</button>
        <Sheet
          open
          label="옵션"
          footer={<button disabled>보낼 수 없음</button>}
          onClose={() => undefined}
        >
          <>
            <button>첫 번째</button>
            <button tabIndex={-1}>프로그램 전용</button>
            <button>두 번째</button>
          </>
        </Sheet>
      </>,
    )

    await user.tab()
    expect(screen.getByRole('button', { name: '첫 번째' })).toHaveFocus()
    await user.tab()
    expect(screen.getByRole('button', { name: '두 번째' })).toHaveFocus()
    // The disabled footer button and the programmatic one are not stops, so the cycle wraps here
    // instead of leaking to 바깥.
    await user.tab()
    expect(screen.getByRole('button', { name: '첫 번째' })).toHaveFocus()
  })
})
