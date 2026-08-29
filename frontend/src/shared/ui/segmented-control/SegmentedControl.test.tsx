import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import { SegmentedControl } from './SegmentedControl'

it('supports click and wrapping arrow-key selection with 44px targets', async () => {
  const onChange = vi.fn()
  render(
    <SegmentedControl
      value="a"
      options={[
        { value: 'a', label: 'A' },
        { value: 'b', label: 'B' },
      ]}
      onChange={onChange}
      ariaLabel="후보"
    />,
  )
  const tabs = screen.getAllByRole('tab')
  expect(tabs[0]).toHaveClass('min-h-11')
  await userEvent.setup().click(tabs[1])
  expect(onChange).toHaveBeenCalledWith('b')
  tabs[0].focus()
  await userEvent.setup().keyboard('{ArrowLeft}')
  expect(onChange).toHaveBeenLastCalledWith('b')
})
