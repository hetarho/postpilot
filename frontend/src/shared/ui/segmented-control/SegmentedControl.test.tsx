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
  expect(tabs[0]).toHaveClass('min-h-10', 'pointer-coarse:min-h-11')
  await userEvent.setup().click(tabs[1])
  expect(onChange).toHaveBeenCalledWith('b')
  tabs[0].focus()
  await userEvent.setup().keyboard('{ArrowLeft}')
  expect(onChange).toHaveBeenLastCalledWith('b')
})

// Between the editor's back button and its delete, the three stations and their chevrons were
// wider than a phone leaves them; centred, they spilled out of both ends, so 글 생성 slid under
// the back button and 글 완성 under the delete.
it('fits the stations to the room their row leaves them', () => {
  render(
    <SegmentedControl
      value="generate"
      variant="steps"
      options={[
        { value: 'generate', label: '글 생성' },
        { value: 'refine', label: '글 다듬기' },
        { value: 'finish', label: '글 완성' },
      ]}
      onChange={() => {}}
      ariaLabel="글 단계"
    />,
  )

  const bar = screen.getByRole('tablist', { name: '글 단계' })
  expect(bar).toHaveClass('@container', 'justify-center-safe')
  expect(bar).not.toHaveClass('justify-center')
  for (const tab of screen.getAllByRole('tab'))
    expect(tab).toHaveClass('@max-[17rem]:px-2.5', '@max-[15.5rem]:px-2')
  const chevrons = bar.querySelectorAll('svg')
  expect(chevrons).toHaveLength(2)
  for (const chevron of chevrons)
    expect(chevron).toHaveClass('@max-[15.5rem]:size-3', '@max-[13.5rem]:hidden')
})
