import { act, cleanup, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { SAVE_STATUS_SETTLED_MS } from '@/shared/config'
import type { SaveState } from './draft-queue'
import { useSaveStatus } from './useSaveStatus'

function Probe({ state }: { state: SaveState }) {
  const status = useSaveStatus(state)
  return <p data-state={status.state}>{status.label}</p>
}

beforeEach(() => vi.useFakeTimers())
afterEach(() => {
  vi.useRealTimers()
  cleanup()
})

describe('useSaveStatus', () => {
  it('says nothing at all for an untouched editor', () => {
    const { container } = render(<Probe state="idle" />)
    expect(container.querySelector('p')).toHaveAttribute('data-state', 'quiet')
    expect(container.querySelector('p')).toBeEmptyDOMElement()
  })

  // The queue's own `saved` holds for the life of a queue that has ever saved, so without this
  // 저장됨 never came down and the post's status could never reach the line (A4).
  it('shows 저장됨 on a completed save and then goes quiet', () => {
    const view = render(<Probe state="saving" />)
    const line = () => view.container.querySelector('p')
    expect(screen.getByText('저장하는 중…')).toBeInTheDocument()

    view.rerender(<Probe state="saved" />)
    expect(screen.getByText('저장됨')).toBeInTheDocument()

    act(() => void vi.advanceTimersByTime(SAVE_STATUS_SETTLED_MS - 1))
    expect(screen.getByText('저장됨')).toBeInTheDocument()

    act(() => void vi.advanceTimersByTime(1))
    expect(screen.queryByText('저장됨')).not.toBeInTheDocument()
    expect(line()).toHaveAttribute('data-state', 'quiet')
    expect(line()).toBeEmptyDOMElement()
  })

  it('re-arms for the next save, so a settled line speaks again', () => {
    const view = render(<Probe state="saved" />)
    act(() => void vi.advanceTimersByTime(SAVE_STATUS_SETTLED_MS))
    expect(screen.queryByText('저장됨')).not.toBeInTheDocument()

    view.rerender(<Probe state="dirty" />)
    expect(screen.getByText('저장 대기 중')).toBeInTheDocument()
    view.rerender(<Probe state="saved" />)
    expect(screen.getByText('저장됨')).toBeInTheDocument()
  })

  it('never settles a state that is still true', () => {
    const { container } = render(<Probe state="error" />)
    act(() => void vi.advanceTimersByTime(SAVE_STATUS_SETTLED_MS * 3))
    expect(container.querySelector('p')).toHaveAttribute('data-state', 'error')
  })
})
