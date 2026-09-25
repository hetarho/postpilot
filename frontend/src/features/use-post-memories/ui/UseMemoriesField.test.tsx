import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { UseMemoriesField } from './UseMemoriesField'

describe('기억 사용', () => {
  // POST-71/MEM-18: default off, and the box carries its name and nothing else.
  it('reads its value and starts off', () => {
    render(<UseMemoriesField checked={false} onChange={() => {}} />)
    const box = screen.getByRole('checkbox', { name: '기억 사용' })
    expect(box).not.toBeChecked()
    expect(box).not.toHaveAccessibleDescription()
  })

  it('shows a checked value as checked', () => {
    render(<UseMemoriesField checked onChange={() => {}} />)
    expect(screen.getByRole('checkbox', { name: '기억 사용' })).toBeChecked()
  })

  // POST-89: a control of the brief's run-options form — it reports the toggle, and the brief's
  // 저장 saves it with the rest of the set. It renders no request of its own, so it needs no
  // transport at all.
  it('reports the toggle both ways and saves nothing', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    const { rerender } = render(<UseMemoriesField checked={false} onChange={onChange} />)

    await user.click(screen.getByRole('checkbox', { name: '기억 사용' }))
    expect(onChange).toHaveBeenLastCalledWith(true)

    rerender(<UseMemoriesField checked onChange={onChange} />)
    await user.click(screen.getByRole('checkbox', { name: '기억 사용' }))
    expect(onChange).toHaveBeenLastCalledWith(false)
    expect(onChange).toHaveBeenCalledTimes(2)
  })

  it('is disabled for a reason the caller states', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<UseMemoriesField checked={false} onChange={onChange} disabled />)

    const box = screen.getByRole('checkbox', { name: '기억 사용' })
    expect(box).toBeDisabled()
    expect(box).not.toHaveAccessibleDescription()
    await user.click(box)
    expect(onChange).not.toHaveBeenCalled()
  })
})
