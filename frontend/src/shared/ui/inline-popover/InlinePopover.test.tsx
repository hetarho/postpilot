import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { InlinePopover } from './InlinePopover'
import { ANCHORED_PANEL_HOVER_CLOSE_MS } from '../anchored-panel/config'
import { FINE_HOVER_MEDIA_QUERY } from '../media-query/useMediaQuery'
import { Popover } from '../popover/Popover'
import { Sheet } from '../sheet/Sheet'

function Sentence({ onChoose = () => undefined }: { onChoose?: () => void }) {
  return (
    <p>
      이번 여름{' '}
      <InlinePopover
        label="바꿔 쓸 표현"
        panel={(close) => (
          <button
            onClick={() => {
              onChoose()
              close()
            }}
          >
            솔직한 방문기
          </button>
        )}
      >
        솔직 후기
      </InlinePopover>{' '}
      를 남겨요
    </p>
  )
}

/** The default `setup.ts` stub matches only reduced motion, so hover opens nothing. These cases
 *  stand in a mouse or trackpad, the way Popover's `sm:` case stands in a wide screen. */
async function withFineHover(run: () => Promise<void>) {
  const original = window.matchMedia
  window.matchMedia = ((query: string) => ({
    matches: query === FINE_HOVER_MEDIA_QUERY,
    media: query,
    onchange: null,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    addListener: () => undefined,
    removeListener: () => undefined,
    dispatchEvent: () => false,
  })) as typeof window.matchMedia
  try {
    await run()
  } finally {
    window.matchMedia = original
  }
}

function rectAt(top: number): DOMRect {
  return { top, bottom: top + 24, left: 40, right: 120, width: 80, height: 24 } as DOMRect
}

describe('InlinePopover', () => {
  it('sits inside the prose as a native button that names what it opens', () => {
    render(<Sentence />)
    const trigger = screen.getByRole('button', { name: '솔직 후기' })
    expect(trigger).toHaveAttribute('type', 'button')
    expect(trigger).toHaveAttribute('aria-haspopup', 'dialog')
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    expect(trigger).not.toHaveAttribute('aria-controls')
    expect(trigger).toHaveClass('inline', 'whitespace-normal', 'text-left', 'break-words')
  })

  it('opens on a press into a portalled panel and moves focus into it', async () => {
    const user = userEvent.setup()
    render(<Sentence />)
    const trigger = screen.getByRole('button', { name: '솔직 후기' })
    await user.click(trigger)

    const panel = screen.getByRole('dialog', { name: '바꿔 쓸 표현' })
    expect(panel.parentElement).toBe(document.body)
    expect(panel).toHaveAttribute('data-anchored-panel')
    expect(panel).toHaveClass('fixed', 'z-overlay-panel', 'bg-surface-overlay')
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
    expect(trigger).toHaveAttribute('aria-controls', panel.id)
    await waitFor(() => expect(screen.getByRole('button', { name: '솔직한 방문기' })).toHaveFocus())

    // A second press on the trigger closes it again.
    await user.click(trigger)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('toggles from the keyboard with Enter and Space', async () => {
    const user = userEvent.setup()
    render(<Sentence />)
    const trigger = screen.getByRole('button', { name: '솔직 후기' })
    trigger.focus()
    await user.keyboard('{Enter}')
    expect(screen.getByRole('dialog', { name: '바꿔 쓸 표현' })).toBeInTheDocument()
    trigger.focus()
    await user.keyboard(' ')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    await user.keyboard(' ')
    expect(screen.getByRole('dialog', { name: '바꿔 쓸 표현' })).toBeInTheDocument()
  })

  it('opens nothing on hover without a fine pointer', async () => {
    const user = userEvent.setup()
    render(<Sentence />)
    await user.hover(screen.getByRole('button', { name: '솔직 후기' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('opens on hover under a fine pointer without moving focus, and closes after the delay', async () => {
    await withFineHover(async () => {
      const user = userEvent.setup()
      render(<Sentence />)
      const trigger = screen.getByRole('button', { name: '솔직 후기' })
      await user.hover(trigger)
      expect(screen.getByRole('dialog', { name: '바꿔 쓸 표현' })).toBeInTheDocument()
      expect(document.body).toHaveFocus()

      await user.unhover(trigger)
      // Still open while the pointer crosses to the panel.
      expect(screen.getByRole('dialog', { name: '바꿔 쓸 표현' })).toBeInTheDocument()
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument(), {
        timeout: ANCHORED_PANEL_HOVER_CLOSE_MS * 10,
      })
    })
  })

  it('stays open while the pointer rests on the panel, and a press there pins it', async () => {
    await withFineHover(async () => {
      const user = userEvent.setup()
      render(<Sentence />)
      await user.hover(screen.getByRole('button', { name: '솔직 후기' }))
      const panel = screen.getByRole('dialog', { name: '바꿔 쓸 표현' })
      await user.hover(panel)
      await new Promise((resolve) => setTimeout(resolve, ANCHORED_PANEL_HOVER_CLOSE_MS * 2))
      expect(panel).toBeInTheDocument()

      await user.pointer({ keys: '[MouseLeft]', target: panel })
      await user.unhover(panel)
      await new Promise((resolve) => setTimeout(resolve, ANCHORED_PANEL_HOVER_CLOSE_MS * 2))
      expect(screen.getByRole('dialog', { name: '바꿔 쓸 표현' })).toBeInTheDocument()
    })
  })

  it('closes only itself on Escape inside a popover, then Escape closes the popover', async () => {
    const user = userEvent.setup()
    render(<Popover label="글쓰기 옵션">{() => <Sentence />}</Popover>)
    await user.click(screen.getByRole('button', { name: '글쓰기 옵션' }))
    const trigger = screen.getByRole('button', { name: '솔직 후기' })
    await user.click(trigger)
    await waitFor(() => expect(screen.getByRole('button', { name: '솔직한 방문기' })).toHaveFocus())

    await user.keyboard('{Escape}')
    expect(screen.queryByRole('dialog', { name: '바꿔 쓸 표현' })).not.toBeInTheDocument()
    expect(screen.getByRole('dialog', { name: '글쓰기 옵션' })).toBeInTheDocument()
    await waitFor(() => expect(trigger).toHaveFocus())

    await user.keyboard('{Escape}')
    expect(screen.queryByRole('dialog', { name: '글쓰기 옵션' })).not.toBeInTheDocument()
  })

  it('closes only itself on Escape inside a sheet', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(
      <Sheet open label="글 설정" onClose={onClose}>
        <Sentence />
      </Sheet>,
    )
    const trigger = screen.getByRole('button', { name: '솔직 후기' })
    await user.click(trigger)
    await waitFor(() => expect(screen.getByRole('button', { name: '솔직한 방문기' })).toHaveFocus())

    await user.keyboard('{Escape}')
    expect(screen.queryByRole('dialog', { name: '바꿔 쓸 표현' })).not.toBeInTheDocument()
    expect(onClose).not.toHaveBeenCalled()
    await waitFor(() => expect(trigger).toHaveFocus())

    await user.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('closes on an outside press', async () => {
    const user = userEvent.setup()
    render(<Sentence />)
    await user.click(screen.getByRole('button', { name: '솔직 후기' }))
    expect(screen.getByRole('dialog', { name: '바꿔 쓸 표현' })).toBeInTheDocument()
    await user.click(document.body)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('hands Tab back to the trigger so the traversal continues in the flow', async () => {
    const user = userEvent.setup()
    render(
      <>
        <Sentence />
        <button>다음 컨트롤</button>
      </>,
    )
    await user.click(screen.getByRole('button', { name: '솔직 후기' }))
    await waitFor(() => expect(screen.getByRole('button', { name: '솔직한 방문기' })).toHaveFocus())

    await user.tab()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '다음 컨트롤' })).toHaveFocus()
  })

  it('closes once a scroll carries its trigger off screen, returning focus it held', async () => {
    const user = userEvent.setup()
    render(<Sentence />)
    const trigger = screen.getByRole('button', { name: '솔직 후기' })
    trigger.getBoundingClientRect = () => rectAt(300)
    await user.click(trigger)
    await waitFor(() => expect(screen.getByRole('button', { name: '솔직한 방문기' })).toHaveFocus())

    trigger.getBoundingClientRect = () => rectAt(-80)
    window.dispatchEvent(new Event('scroll'))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    await waitFor(() => expect(trigger).toHaveFocus())
  })

  it('passes its close to the panel', async () => {
    const user = userEvent.setup()
    const onChoose = vi.fn()
    render(<Sentence onChoose={onChoose} />)
    await user.click(screen.getByRole('button', { name: '솔직 후기' }))
    await user.click(screen.getByRole('button', { name: '솔직한 방문기' }))
    expect(onChoose).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('button', { name: '솔직 후기' })).toHaveFocus())
  })
})
