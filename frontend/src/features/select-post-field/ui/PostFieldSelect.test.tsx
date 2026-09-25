import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import { PostFieldSelect } from './PostFieldSelect'

afterEach(cleanup)

/** The field's chips, each a glyph and a name. */
const chips = () => within(screen.getByRole('group', { name: '분야' })).getAllByRole('button')

// The chips render in place under the field's name: the brief they sit in is itself a popover.
it('offers 없음 and then the nine 분야 in catalogue order, under a visible label', () => {
  render(<PostFieldSelect value="" onChange={() => {}} />)

  expect(screen.getByText('분야')).not.toHaveClass('sr-only')
  expect(screen.queryByRole('button', { name: /^분야 / })).toBeNull()
  expect(chips().map((chip) => chip.textContent)).toEqual([
    '없음',
    '맛집',
    '카페',
    '국내여행',
    '패션·미용',
    '상품리뷰',
    '육아·결혼',
    '반려동물',
    '인테리어·DIY',
    '일상·생각',
  ])
  // Exactly one is pressed — the current choice.
  expect(chips().filter((chip) => chip.getAttribute('aria-pressed') === 'true')).toEqual([
    screen.getByRole('button', { name: '없음', pressed: true }),
  ])
  // Every chip wears a glyph beside its name.
  for (const chip of chips()) expect(chip.querySelector('svg')).not.toBeNull()
})

// POST-89: a control of the brief's run-options form — it reports the choice, and the brief's
// 저장 saves it with the rest of the set.
it('reports a chip as the choice and saves nothing', async () => {
  const user = userEvent.setup()
  const onChange = vi.fn()
  const { rerender } = render(<PostFieldSelect value="" onChange={onChange} />)

  await user.click(screen.getByRole('button', { name: '카페' }))
  expect(onChange).toHaveBeenCalledWith('cafe')

  // The caller's value moves the one pressed chip.
  rerender(<PostFieldSelect value="cafe" onChange={onChange} />)
  expect(screen.getByRole('button', { name: '카페', pressed: true })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '없음', pressed: false })).toBeInTheDocument()
  // Pressing the current choice again reports nothing.
  await user.click(screen.getByRole('button', { name: '카페' }))
  expect(onChange).toHaveBeenCalledTimes(1)
  await user.click(screen.getByRole('button', { name: '없음' }))
  expect(onChange).toHaveBeenLastCalledWith('')
})

it('is disabled for a reason the caller states, adding none of its own', () => {
  render(<PostFieldSelect value="cafe" disabled onChange={() => {}} />)

  for (const chip of chips()) expect(chip).toBeDisabled()
  expect(screen.queryByRole('alert')).toBeNull()
})
