import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { SelectableText } from './SelectableText'

afterEach(() => {
  window.getSelection()?.removeAllRanges()
  cleanup()
})

describe('SelectableText', () => {
  it('keeps exact selectable text and activates on a tap, Enter and Space', async () => {
    const activate = vi.fn()
    const user = userEvent.setup()
    render(<SelectableText onActivate={activate}>제주 🌊 그대로</SelectableText>)
    const text = screen.getByRole('button', { name: '제주 🌊 그대로' })
    expect(text.tagName).toBe('SPAN')
    expect(text).toHaveClass('select-text')
    expect(text.textContent).toBe('제주 🌊 그대로')
    await user.click(text)
    expect(text).toHaveFocus()
    await user.keyboard('{Enter} ')
    expect(activate).toHaveBeenCalledTimes(3)
  })

  it('allows dragging across neighboring phrases and copying without activation', () => {
    const activate = vi.fn()
    render(
      <p>
        <SelectableText onActivate={activate}>첫 문구</SelectableText>{' '}
        <SelectableText onActivate={activate}>다음 문구</SelectableText>
      </p>,
    )
    const first = screen.getByRole('button', { name: '첫 문구' })
    const last = screen.getByRole('button', { name: '다음 문구' })
    const range = document.createRange()
    range.setStart(first.firstChild!, 0)
    range.setEnd(last.firstChild!, 5)
    window.getSelection()!.addRange(range)
    fireEvent.mouseUp(last)
    fireEvent.click(last)
    expect(window.getSelection()!.toString()).toBe('첫 문구 다음 문구')
    expect(activate).not.toHaveBeenCalled()
  })

  it('does not activate after a touch long press creates a selection', () => {
    const activate = vi.fn()
    render(<SelectableText onActivate={activate}>복사할 문구</SelectableText>)
    const text = screen.getByRole('button', { name: '복사할 문구' })
    fireEvent.pointerDown(text, { pointerType: 'touch' })
    const range = document.createRange()
    range.selectNodeContents(text)
    window.getSelection()!.addRange(range)
    fireEvent.pointerUp(text, { pointerType: 'touch' })
    fireEvent.click(text)
    expect(window.getSelection()!.toString()).toBe('복사할 문구')
    expect(activate).not.toHaveBeenCalled()
  })

  it('preserves an existing copy selection before pointerdown can collapse it', () => {
    const activate = vi.fn()
    render(<SelectableText onActivate={activate}>서울😀</SelectableText>)
    const text = screen.getByRole('button', { name: '서울😀' })
    const range = document.createRange()
    range.selectNodeContents(text)
    window.getSelection()!.addRange(range)
    expect(fireEvent.pointerDown(text, { pointerType: 'mouse' })).toBe(false)
    expect(window.getSelection()!.toString()).toBe('서울😀')
    // Even if a platform clears it later, the press still began as a copy selection.
    window.getSelection()!.removeAllRanges()
    fireEvent.click(text)
    expect(activate).not.toHaveBeenCalled()
    fireEvent.pointerDown(text, { pointerType: 'mouse' })
    fireEvent.click(text)
    expect(activate).toHaveBeenCalledOnce()
  })

  it('does not let a selection elsewhere block an intentional tap or keyboard activation', async () => {
    const activate = vi.fn()
    const user = userEvent.setup()
    render(
      <>
        <p>다른 글</p>
        <SelectableText onActivate={activate}>선택한 출처</SelectableText>
      </>,
    )
    const range = document.createRange()
    range.selectNodeContents(screen.getByText('다른 글'))
    window.getSelection()!.addRange(range)
    const text = screen.getByRole('button', { name: '선택한 출처' })
    fireEvent.click(text)
    expect(activate).toHaveBeenCalledOnce()
    range.selectNodeContents(text)
    window.getSelection()!.removeAllRanges()
    window.getSelection()!.addRange(range)
    text.focus()
    await user.keyboard('{Enter}')
    expect(activate).toHaveBeenCalledTimes(2)
  })
})
