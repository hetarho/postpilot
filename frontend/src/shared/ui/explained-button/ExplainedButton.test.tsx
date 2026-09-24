import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { ExplainedButton } from './ExplainedButton'
import { FINE_HOVER_MEDIA_QUERY } from '../media-query/useMediaQuery'

const REASON = '작성 A/B 모델 두 개를 선택하세요.'

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

describe('ExplainedButton', () => {
  it('stays focusable while it explains, described by its reason, and never acts', async () => {
    const user = userEvent.setup()
    const onClick = vi.fn()
    render(
      <ExplainedButton disabled reason={REASON} onClick={onClick}>
        A/B 비교
      </ExplainedButton>,
    )
    const button = screen.getByRole('button', { name: 'A/B 비교' })
    expect(button).toBeEnabled()
    expect(button).toHaveAttribute('aria-disabled', 'true')
    expect(button).toHaveAccessibleDescription(REASON)
    expect(bubble()).toBeNull()

    await user.click(button)
    expect(onClick).not.toHaveBeenCalled()
    const tip = bubble()
    expect(tip).toHaveTextContent(REASON)
    expect(tip).toHaveAttribute('aria-hidden', 'true')
    expect(tip?.parentElement).toBe(document.body)

    await user.click(button)
    expect(bubble()).toBeNull()
  })

  it('opens from the keyboard and closes on Escape', async () => {
    const user = userEvent.setup()
    render(
      <ExplainedButton disabled reason={REASON}>
        A/B 비교
      </ExplainedButton>,
    )
    await user.tab()
    expect(screen.getByRole('button', { name: 'A/B 비교' })).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(bubble()).toHaveTextContent(REASON)
    await user.keyboard('{Escape}')
    expect(bubble()).toBeNull()
  })

  it('opens under a resting fine pointer', async () => {
    await withFineHover(async () => {
      const user = userEvent.setup()
      render(
        <ExplainedButton disabled reason={REASON}>
          A/B 비교
        </ExplainedButton>,
      )
      await user.hover(screen.getByRole('button', { name: 'A/B 비교' }))
      expect(bubble()).toHaveTextContent(REASON)
    })
  })

  it('is an ordinary button when enabled, pending or given no reason', async () => {
    const user = userEvent.setup()
    const onClick = vi.fn()
    const { rerender } = render(
      <ExplainedButton reason={REASON} onClick={onClick}>
        A/B 비교
      </ExplainedButton>,
    )
    const button = screen.getByRole('button', { name: 'A/B 비교' })
    expect(button).not.toHaveAttribute('aria-disabled')
    expect(button).not.toHaveAccessibleDescription()
    await user.click(button)
    expect(onClick).toHaveBeenCalledTimes(1)
    expect(bubble()).toBeNull()

    rerender(<ExplainedButton disabled>A/B 비교</ExplainedButton>)
    expect(screen.getByRole('button', { name: 'A/B 비교' })).toBeDisabled()

    rerender(
      <ExplainedButton disabled pending reason={REASON}>
        A/B 비교
      </ExplainedButton>,
    )
    expect(screen.getByRole('button', { name: 'A/B 비교' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'A/B 비교' })).not.toHaveAccessibleDescription()
  })

  it('takes an open tip down once the reason is gone', async () => {
    const user = userEvent.setup()
    const { rerender } = render(
      <ExplainedButton disabled reason={REASON}>
        A/B 비교
      </ExplainedButton>,
    )
    await user.click(screen.getByRole('button', { name: 'A/B 비교' }))
    expect(bubble()).not.toBeNull()

    rerender(<ExplainedButton>A/B 비교</ExplainedButton>)
    expect(bubble()).toBeNull()
    // Nor does it come back by itself with the next reason.
    rerender(
      <ExplainedButton disabled reason={REASON}>
        A/B 비교
      </ExplainedButton>,
    )
    expect(bubble()).toBeNull()
  })
})
