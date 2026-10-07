import { fireEvent, render, screen } from '@testing-library/react'
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
