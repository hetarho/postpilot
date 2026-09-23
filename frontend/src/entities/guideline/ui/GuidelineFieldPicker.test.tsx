import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import { GuidelineFieldPicker } from './GuidelineFieldPicker'

afterEach(() => cleanup())

it('emits the next set in catalogue order whatever the press order', async () => {
  const user = userEvent.setup()
  const onChange = vi.fn()
  render(<GuidelineFieldPicker value={['pets']} onChange={onChange} legend="적용할 분야" />)

  await user.click(screen.getByLabelText('맛집'))
  expect(onChange).toHaveBeenLastCalledWith(['restaurant', 'pets'])
  await user.click(screen.getByLabelText('반려동물'))
  expect(onChange).toHaveBeenLastCalledWith([])
})

it('names the list for assistive tech, and shows the legend only when asked', () => {
  const { rerender } = render(
    <GuidelineFieldPicker value={[]} onChange={() => {}} legend="적용할 분야" />,
  )
  expect(screen.getByRole('group', { name: '적용할 분야' })).toBeInTheDocument()
  expect(screen.getByText('적용할 분야')).toHaveClass('sr-only')

  rerender(
    <GuidelineFieldPicker value={[]} onChange={() => {}} legend="적용할 분야" legendVisible />,
  )
  expect(screen.getByText('적용할 분야')).not.toHaveClass('sr-only')
})

it('holds every box while disabled', async () => {
  const user = userEvent.setup()
  const onChange = vi.fn()
  render(
    <GuidelineFieldPicker value={['cafe']} onChange={onChange} legend="적용할 분야" disabled />,
  )

  for (const box of screen.getAllByRole('checkbox')) expect(box).toBeDisabled()
  await user.click(screen.getByLabelText('맛집'))
  expect(onChange).not.toHaveBeenCalled()
})
