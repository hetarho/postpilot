import { useState } from 'react'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { FieldLabel } from '../field/FieldLabel'
import { Listbox } from './Listbox'

type Fruit = 'apple' | 'pear' | 'plum'

function Harness({ onChanged }: { onChanged?: (value: Fruit) => void }) {
  const [value, setValue] = useState<Fruit>('pear')
  return (
    <>
      <FieldLabel id="fruit-label" htmlFor="fruit">
        과일
      </FieldLabel>
      <Listbox<Fruit>
        id="fruit"
        aria-labelledby="fruit-label"
        value={value}
        options={[
          { value: 'apple', label: '사과' },
          { value: 'pear', label: '배' },
          { value: 'plum', label: '자두', disabled: true },
        ]}
        onChange={(next) => {
          setValue(next)
          onChanged?.(next)
        }}
      />
    </>
  )
}

afterEach(cleanup)

describe('Listbox', () => {
  it('names itself with its label and its current option, and wears the field well', () => {
    render(<Harness />)
    const trigger = screen.getByRole('combobox', { name: '과일 배' })
    expect(trigger).toHaveClass('bg-field-bg', 'min-h-11')
    expect(trigger).toHaveAttribute('aria-haspopup', 'listbox')
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('opens an app-drawn panel whose current option holds focus', async () => {
    const user = userEvent.setup()
    render(<Harness />)

    await user.click(screen.getByRole('combobox', { name: '과일 배' }))

    const panel = screen.getByRole('listbox', { name: '과일' })
    expect(panel).toHaveClass('bg-surface-highest')
    expect(panel).not.toHaveClass('border')
    expect(screen.getAllByRole('option').map((option) => option.textContent)).toEqual([
      '사과',
      '배',
      '자두',
    ])
    expect(screen.getByRole('option', { name: '배', selected: true })).toHaveFocus()
  })

  it('moves with the arrows and Home/End, and selects with Enter', async () => {
    const user = userEvent.setup()
    const onChanged = vi.fn()
    render(<Harness onChanged={onChanged} />)

    const trigger = screen.getByRole('combobox', { name: '과일 배' })
    trigger.focus()
    await user.keyboard('{ArrowDown}')
    expect(screen.getByRole('option', { name: '배' })).toHaveFocus()

    await user.keyboard('{End}')
    expect(screen.getByRole('option', { name: '자두' })).toHaveFocus()
    await user.keyboard('{Home}')
    expect(screen.getByRole('option', { name: '사과' })).toHaveFocus()

    await user.keyboard('{Enter}')
    expect(onChanged).toHaveBeenCalledWith('apple')
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: '과일 사과' })).toHaveFocus()
  })

  it('refuses a disabled option without closing over the reason it is listed', async () => {
    const user = userEvent.setup()
    const onChanged = vi.fn()
    render(<Harness onChanged={onChanged} />)

    await user.click(screen.getByRole('combobox', { name: '과일 배' }))
    await user.click(screen.getByRole('option', { name: '자두' }))

    expect(onChanged).not.toHaveBeenCalled()
    expect(screen.getByRole('listbox')).toBeInTheDocument()
  })

  it('closes on Escape and on an outside press, returning focus to the trigger', async () => {
    const user = userEvent.setup()
    render(
      <div>
        <Harness />
        <button>바깥</button>
      </div>,
    )

    const trigger = screen.getByRole('combobox', { name: '과일 배' })
    await user.click(trigger)
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
    expect(trigger).toHaveFocus()

    await user.click(trigger)
    await user.click(screen.getByRole('button', { name: '바깥' }))
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('shows the placeholder for a value no option carries, and reports invalid state', () => {
    render(
      <Listbox<string>
        aria-label="모델"
        aria-invalid
        value=""
        placeholder="선택하세요"
        options={[{ value: 'a', label: 'A' }]}
        onChange={() => undefined}
      />,
    )
    const trigger = screen.getByRole('combobox', { name: '모델' })
    expect(trigger).toHaveTextContent('선택하세요')
    expect(trigger).toHaveAttribute('aria-invalid', 'true')
  })

  it('round-trips a numeric value without a stringly-typed cast', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(
      <Listbox<number>
        aria-label="제목 단계"
        value={2}
        options={[
          { value: 2, label: 'H2' },
          { value: 3, label: 'H3' },
        ]}
        onChange={onChange}
      />,
    )
    await user.click(screen.getByRole('combobox', { name: '제목 단계' }))
    await user.click(screen.getByRole('option', { name: 'H3' }))
    expect(onChange).toHaveBeenCalledWith(3)
  })

  it('does not open while disabled', async () => {
    const user = userEvent.setup()
    render(
      <Listbox<string>
        aria-label="모델"
        disabled
        value="a"
        options={[{ value: 'a', label: 'A' }]}
        onChange={() => undefined}
      />,
    )
    await user.click(screen.getByRole('combobox', { name: '모델' }))
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })
})
