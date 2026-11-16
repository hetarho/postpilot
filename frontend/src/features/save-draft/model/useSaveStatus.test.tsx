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
  // the line never came down and the post's status could never reach it (A4). Waiting, running
  // and done saves say nothing on screen (owner 2026-11-16); the resolved state still settles.
  it('resolves a completed save silently and then settles quiet', () => {
    const view = render(<Probe state="saving" />)
    const line = () => view.container.querySelector('p')
    expect(line()).toHaveAttribute('data-state', 'saving')
    expect(line()).toBeEmptyDOMElement()

    view.rerender(<Probe state="saved" />)
    expect(line()).toHaveAttribute('data-state', 'saved')
    expect(line()).toBeEmptyDOMElement()

    act(() => void vi.advanceTimersByTime(SAVE_STATUS_SETTLED_MS - 1))
    expect(line()).toHaveAttribute('data-state', 'saved')

    act(() => void vi.advanceTimersByTime(1))
    expect(line()).toHaveAttribute('data-state', 'quiet')
    expect(line()).toBeEmptyDOMElement()
  })

  it('re-arms for the next save, so a settled line resolves again', () => {
    const view = render(<Probe state="saved" />)
    const line = () => view.container.querySelector('p')
    act(() => void vi.advanceTimersByTime(SAVE_STATUS_SETTLED_MS))
    expect(line()).toHaveAttribute('data-state', 'quiet')

    view.rerender(<Probe state="dirty" />)
    expect(line()).toHaveAttribute('data-state', 'dirty')
    expect(line()).toBeEmptyDOMElement()
    view.rerender(<Probe state="saved" />)
    expect(line()).toHaveAttribute('data-state', 'saved')
    expect(line()).toBeEmptyDOMElement()
  })

  it('still speaks for a save that is failing or refused', () => {
    const view = render(<Probe state="error" />)
    expect(screen.getByText('저장하지 못했어요 · 다시 시도 중')).toBeInTheDocument()
    view.rerender(<Probe state="refused" />)
    expect(screen.getByText('저장이 거절됐어요')).toBeInTheDocument()
  })

  it('never settles a state that is still true', () => {
    const { container } = render(<Probe state="error" />)
    act(() => void vi.advanceTimersByTime(SAVE_STATUS_SETTLED_MS * 3))
    expect(container.querySelector('p')).toHaveAttribute('data-state', 'error')
  })
})
