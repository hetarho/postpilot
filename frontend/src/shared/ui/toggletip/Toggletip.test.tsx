import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Toggletip } from './Toggletip'
import { ANCHORED_PANEL_HOVER_CLOSE_MS } from '../anchored-panel/config'
import { FINE_HOVER_MEDIA_QUERY } from '../media-query/useMediaQuery'
import { Popover } from '../popover/Popover'
import { Sheet } from '../sheet/Sheet'

const TIP = '본문에서 가장 많이 쓴 명사가 차지하는 비율이에요.'

/** A checkbox row as the writing brief will lay one out: the tip BESIDE the label, never in it. */
function Row() {
  return (
    <div>
      <label>
        <input type="checkbox" /> 반복 줄이기
      </label>
      <Toggletip label="반복 줄이기 설명">{TIP}</Toggletip>
    </div>
  )
}

function bubble(): HTMLElement | null {
  return document.body.querySelector<HTMLElement>('[data-anchored-panel]')
}

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

describe('Toggletip', () => {
  it('is an icon button named for what it explains', () => {
    render(<Row />)
    const button = screen.getByRole('button', { name: '반복 줄이기 설명' })
    expect(button).toHaveAttribute('aria-expanded', 'false')
    // Its bubble is aria-hidden, so there is nothing for aria-controls to point at.
    expect(button).not.toHaveAttribute('aria-controls')
    expect(button).toHaveClass('size-10', 'pointer-coarse:size-11')
  })

  it('opens on a press, leaves the checkbox beside it unticked, and keeps focus on the button', async () => {
    const user = userEvent.setup()
    render(<Row />)
    const button = screen.getByRole('button', { name: '반복 줄이기 설명' })
    await user.click(button)

    expect(button).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByRole('checkbox')).not.toBeChecked()
    expect(button).toHaveFocus()
    const tip = bubble()
    expect(tip).toHaveTextContent(TIP)
    expect(tip).toHaveAttribute('aria-hidden', 'true')
    expect(tip?.parentElement).toBe(document.body)

    await user.click(button)
    expect(bubble()).toBeNull()
  })

  it('mirrors its text into a status region that is always mounted', async () => {
    const user = userEvent.setup()
    render(<Row />)
    const status = screen.getByRole('status')
    expect(status).toHaveClass('sr-only')
    expect(status).toBeEmptyDOMElement()

    await user.click(screen.getByRole('button', { name: '반복 줄이기 설명' }))
    expect(screen.getByRole('status')).toBe(status)
    expect(status).toHaveTextContent(TIP)
  })

  it('opens on hover only under a fine pointer', async () => {
    const user = userEvent.setup()
    const { unmount } = render(<Row />)
    await user.hover(screen.getByRole('button', { name: '반복 줄이기 설명' }))
    expect(bubble()).toBeNull()
    unmount()

    await withFineHover(async () => {
      render(<Row />)
      const button = screen.getByRole('button', { name: '반복 줄이기 설명' })
      await user.hover(button)
      expect(bubble()).toHaveTextContent(TIP)
      await user.unhover(button)
      await waitFor(() => expect(bubble()).toBeNull(), {
        timeout: ANCHORED_PANEL_HOVER_CLOSE_MS * 10,
      })
    })
  })

  it('closes only itself on Escape inside a popover', async () => {
    const user = userEvent.setup()
    render(<Popover label="글쓰기 옵션">{() => <Row />}</Popover>)
    await user.click(screen.getByRole('button', { name: '글쓰기 옵션' }))
    const button = screen.getByRole('button', { name: '반복 줄이기 설명' })
    await user.click(button)
    expect(bubble()).not.toBeNull()

    await user.keyboard('{Escape}')
    expect(bubble()).toBeNull()
    expect(screen.getByRole('dialog', { name: '글쓰기 옵션' })).toBeInTheDocument()
    await waitFor(() => expect(button).toHaveFocus())

    await user.keyboard('{Escape}')
    expect(screen.queryByRole('dialog', { name: '글쓰기 옵션' })).not.toBeInTheDocument()
  })

  // THEME-32, THEME-30: a rest moves no focus, so Escape reaches the document first; it must close
  // the tip and nothing around it.
  it('closes a hover-opened tip on Escape without moving focus, and only it inside a popover', async () => {
    await withFineHover(async () => {
      const user = userEvent.setup()
      render(<Popover label="글쓰기 옵션">{() => <Row />}</Popover>)
      await user.click(screen.getByRole('button', { name: '글쓰기 옵션' }))
      const box = screen.getByRole('checkbox', { name: '반복 줄이기' })
      await waitFor(() => expect(box).toHaveFocus())

      await user.hover(screen.getByRole('button', { name: '반복 줄이기 설명' }))
      expect(bubble()).not.toBeNull()
      await user.keyboard('{Escape}')
      expect(bubble()).toBeNull()
      expect(screen.getByRole('dialog', { name: '글쓰기 옵션' })).toBeInTheDocument()
      expect(box).toHaveFocus()

      await user.keyboard('{Escape}')
      expect(screen.queryByRole('dialog', { name: '글쓰기 옵션' })).not.toBeInTheDocument()
    })
  })

  it('closes only itself on Escape inside a sheet', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(
      <Sheet open label="글 설정" onClose={onClose}>
        <Row />
      </Sheet>,
    )
    const button = screen.getByRole('button', { name: '반복 줄이기 설명' })
    await user.click(button)
    expect(bubble()).not.toBeNull()

    await user.keyboard('{Escape}')
    expect(bubble()).toBeNull()
    expect(onClose).not.toHaveBeenCalled()

    await user.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('closes on an outside press', async () => {
    const user = userEvent.setup()
    render(<Row />)
    await user.click(screen.getByRole('button', { name: '반복 줄이기 설명' }))
    expect(bubble()).not.toBeNull()
    await user.click(document.body)
    expect(bubble()).toBeNull()
    expect(screen.getByRole('status')).toBeEmptyDOMElement()
  })
})
