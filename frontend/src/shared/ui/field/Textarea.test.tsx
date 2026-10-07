import { act, fireEvent, render, screen } from '@testing-library/react'
import { Textarea } from './Textarea'

describe('textarea writing allocation', () => {
  it('allocates empty space below the measured top and responds to viewport/keyboard resizing', () => {
    Object.defineProperty(window, 'innerHeight', { configurable: true, value: 900 })
    const rect = vi
      .spyOn(HTMLElement.prototype, 'getBoundingClientRect')
      .mockReturnValue({ top: 220 } as DOMRect)
    const { rerender } = render(
      <Textarea
        aria-label="Writing"
        autoGrow
        viewportAllocation={{ reservedBottom: 100 }}
        value=""
        readOnly
      />,
    )
    const field = screen.getByRole('textbox')
    expect(field.style.minHeight).toBe('580px')
    Object.defineProperty(window, 'innerHeight', { configurable: true, value: 600 })
    fireEvent(window, new Event('resize'))
    expect(field.style.minHeight).toBe('280px')
    field.focus()
    rerender(
      <Textarea
        aria-label="Writing"
        autoGrow
        viewportAllocation={{ reservedBottom: 100 }}
        value="short"
        readOnly
      />,
    )
    expect(field).toHaveFocus()
    expect(field).toHaveValue('short')
    rerender(<Textarea aria-label="Writing" autoGrow value="short" readOnly />)
    expect(field.style.minHeight).toBe('')
    rect.mockRestore()
  })
  it('retains normal natural growth and shrinks after deleting content', () => {
    const { rerender } = render(<Textarea aria-label="Writing" autoGrow value="long" readOnly />)
    const field = screen.getByRole('textbox')
    Object.defineProperty(field, 'scrollHeight', { configurable: true, value: 1400 })
    Object.defineProperty(field, 'clientHeight', { configurable: true, value: 1400 })
    rerender(<Textarea aria-label="Writing" autoGrow value="longer" readOnly />)
    expect(field.style.height).toBe('1400px')
    expect(field.style.overflowY).toBe('hidden')
    Object.defineProperty(field, 'scrollHeight', { configurable: true, value: 80 })
    Object.defineProperty(field, 'clientHeight', { configurable: true, value: 80 })
    rerender(<Textarea aria-label="Writing" autoGrow value="short" readOnly />)
    expect(field.style.height).toBe('80px')
    expect(field.style.minHeight).toBe('')
  })
  it('grows beyond its viewport allocation while preserving one document scroller', () => {
    const { rerender } = render(
      <Textarea
        aria-label="Writing"
        viewportAllocation={{ reservedBottom: 100 }}
        value=""
        readOnly
      />,
    )
    const field = screen.getByRole('textbox')
    Object.defineProperty(field, 'scrollHeight', { configurable: true, value: 1800 })
    Object.defineProperty(field, 'clientHeight', { configurable: true, value: 1800 })
    rerender(
      <Textarea
        aria-label="Writing"
        viewportAllocation={{ reservedBottom: 100 }}
        value="very long"
        readOnly
      />,
    )
    expect(field.style.height).toBe('1800px')
    expect(field.style.overflowY).toBe('hidden')
  })
})

it('defers observer-induced height writes, coalesces notifications and cancels them on unmount', () => {
  let deliver!: ResizeObserverCallback
  const disconnect = vi.fn(),
    frames = new Map<number, FrameRequestCallback>()
  let next = 0
  vi.stubGlobal(
    'ResizeObserver',
    class {
      constructor(callback: ResizeObserverCallback) {
        deliver = callback
      }
      observe() {}
      disconnect() {
        disconnect()
      }
    },
  )
  const request = vi.spyOn(window, 'requestAnimationFrame').mockImplementation((callback) => {
    const id = ++next
    frames.set(id, callback)
    return id
  })
  const cancel = vi.spyOn(window, 'cancelAnimationFrame').mockImplementation((id) => {
    frames.delete(id)
  })
  const view = render(<Textarea aria-label="Writing" autoGrow defaultValue="Text" />),
    field = screen.getByRole('textbox')
  Object.defineProperty(field, 'scrollHeight', { configurable: true, value: 240 })
  const before = field.style.height
  act(() => {
    deliver([], {} as ResizeObserver)
    deliver([], {} as ResizeObserver)
  })
  expect(field.style.height).toBe(before)
  expect(request).toHaveBeenCalledTimes(1)
  act(() => {
    const callback = frames.get(1)!
    frames.delete(1)
    callback(0)
  })
  expect(field.style.height).toBe('240px')
  act(() => deliver([], {} as ResizeObserver))
  view.unmount()
  expect(cancel).toHaveBeenCalledWith(2)
  expect(frames.size).toBe(0)
  expect(disconnect).toHaveBeenCalledTimes(1)
  request.mockRestore()
  cancel.mockRestore()
  vi.unstubAllGlobals()
})
