import { useId, useState } from 'react'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import { radioTiles } from './radioTiles'

function Tiles({
  title,
  options,
  initial = 0,
  onSelect,
}: {
  title: string
  options: readonly string[]
  initial?: number
  onSelect?: (index: number) => void
}) {
  const label = useId()
  const [selected, setSelected] = useState(initial)
  const tiles = radioTiles(options.length, selected, (index) => {
    onSelect?.(index)
    setSelected(index)
  })
  return (
    <div {...tiles.group} aria-labelledby={label}>
      <p id={label}>{title}</p>
      {options.map((option, index) => (
        <button key={option} {...tiles.tile(index, 'gap-1')}>
          {option}
        </button>
      ))}
    </div>
  )
}

const tile = (name: string) => screen.getByRole('radio', { name })

it('moves choice and focus together with every arrow, wrapping at either end', async () => {
  const user = userEvent.setup()
  const onSelect = vi.fn()
  render(<Tiles title="Pace" options={['One', 'Two', 'Three']} onSelect={onSelect} />)
  await user.tab()
  expect(tile('One')).toHaveFocus()
  for (const [key, name] of [
    ['{ArrowRight}', 'Two'],
    ['{ArrowDown}', 'Three'],
    ['{ArrowDown}', 'One'],
    ['{ArrowLeft}', 'Three'],
    ['{ArrowUp}', 'Two'],
  ]) {
    await user.keyboard(key)
    expect(tile(name)).toHaveFocus()
    expect(tile(name)).toHaveAttribute('aria-checked', 'true')
    expect(tile(name)).toHaveAttribute('tabindex', '0')
  }
  expect(onSelect.mock.calls).toEqual([[1], [2], [0], [2], [1]])
  expect(tile('One')).toHaveAttribute('aria-checked', 'false')
  expect(tile('One')).toHaveAttribute('tabindex', '-1')
  await user.keyboard('{Home}x')
  expect(onSelect).toHaveBeenCalledTimes(5)
  await user.click(tile('Three'))
  expect(onSelect).toHaveBeenLastCalledWith(2)
  expect(tile('Three')).toHaveAttribute('aria-checked', 'true')
})

it('leaves no tile selected or tabbable at -1 and steps an arrow from before the first', async () => {
  const user = userEvent.setup()
  const onSelect = vi.fn()
  render(<Tiles title="Intro" options={['Off', 'A', 'B', 'C']} initial={-1} onSelect={onSelect} />)
  for (const radio of screen.getAllByRole('radio')) {
    expect(radio).toHaveAttribute('aria-checked', 'false')
    expect(radio).toHaveAttribute('tabindex', '-1')
  }
  tile('B').focus()
  await user.keyboard('{ArrowRight}')
  expect(onSelect).toHaveBeenLastCalledWith(0)
  expect(tile('Off')).toHaveFocus()
})

it('steps back from no selection the same way', async () => {
  const user = userEvent.setup()
  const onSelect = vi.fn()
  render(<Tiles title="Intro" options={['Off', 'A', 'B', 'C']} initial={-1} onSelect={onSelect} />)
  tile('A').focus()
  await user.keyboard('{ArrowLeft}')
  expect(onSelect).toHaveBeenLastCalledWith(2)
  expect(tile('B')).toHaveFocus()
})

it('keeps two groups on one screen apart, each named by its own title', async () => {
  const user = userEvent.setup()
  render(
    <>
      <Tiles title="Intro" options={['A', 'B']} />
      <Tiles title="Outro" options={['A', 'B']} />
    </>,
  )
  const intro = screen.getByRole('radiogroup', { name: 'Intro' })
  const outro = screen.getByRole('radiogroup', { name: 'Outro' })
  within(outro).getByRole('radio', { name: 'A' }).focus()
  await user.keyboard('{ArrowRight}')
  expect(within(outro).getByRole('radio', { name: 'B' })).toHaveFocus()
  expect(within(outro).getByRole('radio', { name: 'B' })).toHaveAttribute('aria-checked', 'true')
  expect(within(intro).getByRole('radio', { name: 'A' })).toHaveAttribute('aria-checked', 'true')
  expect(within(intro).getByRole('radio', { name: 'B' })).toHaveAttribute('aria-checked', 'false')
})

it('follows changed options and styles the selected tile apart from the rest', async () => {
  const user = userEvent.setup()
  const { rerender } = render(<Tiles title="Style" options={['Plain', 'Neon']} initial={1} />)
  rerender(<Tiles title="Style" options={['Plain', 'Neon', 'Film']} initial={1} />)
  expect(screen.getAllByRole('radio')).toHaveLength(3)
  expect(tile('Neon')).toHaveAttribute('aria-checked', 'true')
  expect(tile('Neon')).toHaveAttribute('type', 'button')
  expect(tile('Neon')).toHaveClass('gap-1', 'ring-2', 'bg-row-bg-active')
  expect(tile('Film')).toHaveClass('gap-1', 'hover:bg-row-bg-hover')
  expect(tile('Film')).not.toHaveClass('ring-2')
  tile('Neon').focus()
  await user.keyboard('{ArrowRight}')
  expect(tile('Film')).toHaveFocus()
  await user.keyboard('{ArrowRight}')
  expect(tile('Plain')).toHaveFocus()
})
