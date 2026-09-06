import { useState } from 'react'
import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Stepper } from './Stepper'

function Bounded({
  initial = 1,
  min = 1,
  max = 4,
}: {
  initial?: number
  min?: number
  max?: number
}) {
  const [value, setValue] = useState(initial)
  return (
    <Stepper
      label="가로로 놓을 사진 수"
      value={value}
      min={min}
      max={max}
      onChange={setValue}
      decrementLabel="줄이기"
      incrementLabel="늘리기"
    />
  )
}

const readout = () => screen.getByRole('spinbutton', { name: '가로로 놓을 사진 수' })

describe('Stepper', () => {
  it('announces the value with its bounds', () => {
    render(<Bounded initial={2} />)
    expect(readout()).toHaveAttribute('aria-valuenow', '2')
    expect(readout()).toHaveAttribute('aria-valuemin', '1')
    expect(readout()).toHaveAttribute('aria-valuemax', '4')
    expect(readout()).toHaveTextContent('2')
  })

  it('steps one at a time in both directions', async () => {
    const user = userEvent.setup()
    render(<Bounded initial={2} />)

    await user.click(screen.getByRole('button', { name: '늘리기' }))
    expect(readout()).toHaveAttribute('aria-valuenow', '3')
    await user.click(screen.getByRole('button', { name: '줄이기' }))
    expect(readout()).toHaveAttribute('aria-valuenow', '2')
  })

  // A control that accepts a press and does nothing reads as broken, so the bounds DISABLE
  // rather than clamp silently.
  it('disables each button at its own bound', async () => {
    const user = userEvent.setup()
    render(<Bounded initial={1} />)
    expect(screen.getByRole('button', { name: '줄이기' })).toBeDisabled()
    expect(screen.getByRole('button', { name: '늘리기' })).toBeEnabled()

    await user.click(screen.getByRole('button', { name: '늘리기' }))
    await user.click(screen.getByRole('button', { name: '늘리기' }))
    await user.click(screen.getByRole('button', { name: '늘리기' }))
    expect(readout()).toHaveAttribute('aria-valuenow', '4')
    expect(screen.getByRole('button', { name: '늘리기' })).toBeDisabled()
  })

  // The readout is the spin button, so it takes the keys a native one takes.
  it('steps with the arrow keys and jumps with Home and End', async () => {
    const user = userEvent.setup()
    render(<Bounded initial={2} />)

    readout().focus()
    await user.keyboard('{ArrowUp}')
    expect(readout()).toHaveAttribute('aria-valuenow', '3')
    await user.keyboard('{ArrowDown}{ArrowLeft}')
    expect(readout()).toHaveAttribute('aria-valuenow', '1')
    // Already at the floor: a key press past a bound changes nothing rather than wrapping.
    await user.keyboard('{ArrowDown}')
    expect(readout()).toHaveAttribute('aria-valuenow', '1')

    await user.keyboard('{End}')
    expect(readout()).toHaveAttribute('aria-valuenow', '4')
    await user.keyboard('{Home}')
    expect(readout()).toHaveAttribute('aria-valuenow', '1')
  })

  it('takes no input at all while disabled', async () => {
    const user = userEvent.setup()
    render(
      <Stepper
        label="가로로 놓을 사진 수"
        value={2}
        min={1}
        max={4}
        disabled
        onChange={() => expect.unreachable('a disabled stepper changed its value')}
        decrementLabel="줄이기"
        incrementLabel="늘리기"
      />,
    )
    expect(screen.getByRole('button', { name: '늘리기' })).toBeDisabled()
    expect(screen.getByRole('button', { name: '줄이기' })).toBeDisabled()
    readout().focus()
    await user.keyboard('{ArrowUp}')
  })

  it('formats the value when asked', () => {
    render(
      <Stepper
        label="가로로 놓을 사진 수"
        value={3}
        min={1}
        max={4}
        onChange={() => {}}
        decrementLabel="줄이기"
        incrementLabel="늘리기"
        formatValue={(value) => `${value}장`}
      />,
    )
    expect(readout()).toHaveTextContent('3장')
  })
})
