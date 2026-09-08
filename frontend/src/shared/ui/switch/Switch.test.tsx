import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Switch } from './Switch'

describe('Switch', () => {
  // Announced as on/off rather than checked: that is the whole difference from a Checkbox, and
  // the reason this primitive exists.
  it('is a switch with its state, named by its caller', () => {
    render(<Switch aria-label="데이터 받기" checked={false} onChange={() => {}} />)
    const control = screen.getByRole('switch', { name: '데이터 받기' })
    expect(control).not.toBeChecked()
  })

  it('reports a flip to its caller', async () => {
    const onChange = vi.fn()
    render(<Switch aria-label="데이터 받기" checked={false} onChange={onChange} />)
    await userEvent.click(screen.getByRole('switch'))
    expect(onChange).toHaveBeenCalledTimes(1)
  })

  it('cannot be flipped while disabled', async () => {
    const onChange = vi.fn()
    render(<Switch aria-label="데이터 받기" checked disabled onChange={onChange} />)
    const control = screen.getByRole('switch')
    expect(control).toBeChecked()
    expect(control).toBeDisabled()
    await userEvent.click(control)
    expect(onChange).not.toHaveBeenCalled()
  })

  // The primitive owes the touch target, not the caller (THEME-23): under a coarse pointer the
  // 24px track has to grow past 44px without moving anything around it.
  it('grows its own hit area under a coarse pointer', () => {
    render(<Switch aria-label="데이터 받기" checked={false} onChange={() => {}} />)
    expect(screen.getByRole('switch').className).toContain('pointer-coarse:-inset-2.5')
  })
})
