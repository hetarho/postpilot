import { useState } from 'react'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
import { Languages } from 'lucide-react'
import { Menu } from './Menu'

type Fruit = 'apple' | 'pear' | 'plum'

function Harness({ onChanged }: { onChanged?: (value: Fruit) => void }) {
  const [value, setValue] = useState<Fruit>('pear')
  return (
    <Menu<Fruit>
      label="Fruit"
      triggerDescription={`Current: ${value}`}
      value={value}
      options={[
        { value: 'apple', label: 'Apple' },
        { value: 'pear', label: 'Pear' },
        { value: 'plum', label: 'Plum' },
      ]}
      onChange={(next) => {
        setValue(next)
        onChanged?.(next)
      }}
      triggerIcon={<Languages aria-hidden="true" className="size-6" />}
    />
  )
}

afterEach(cleanup)

describe('Menu', () => {
  it('opens an app-drawn menu from an icon trigger and marks the active option', async () => {
    const user = userEvent.setup()
    render(<Harness />)

    const trigger = screen.getByRole('button', { name: 'Fruit' })
    expect(trigger).toHaveClass('size-11')
    expect(trigger).toHaveAttribute('aria-haspopup', 'menu')
    expect(trigger).toHaveAccessibleDescription('Current: pear')
    await user.click(trigger)

    const menu = screen.getByRole('menu', { name: 'Fruit' })
    expect(menu).toHaveClass('bg-surface-highest')
    expect(menu).not.toHaveClass('border')
    const options = screen.getAllByRole('menuitemradio')
    expect(options.map((option) => option.textContent)).toEqual(['Apple', 'Pear', 'Plum'])
    expect(screen.getByRole('menuitemradio', { name: 'Pear', checked: true })).toHaveFocus()
  })

  it('selects with the keyboard and returns focus to the trigger', async () => {
    const changes: Fruit[] = []
    const user = userEvent.setup()
    render(<Harness onChanged={(value) => changes.push(value)} />)

    const trigger = screen.getByRole('button', { name: 'Fruit' })
    trigger.focus()
    await user.keyboard('{ArrowDown}')
    expect(screen.getByRole('menuitemradio', { name: 'Pear' })).toHaveFocus()

    await user.keyboard('{ArrowDown}')
    expect(screen.getByRole('menuitemradio', { name: 'Plum' })).toHaveFocus()
    await user.keyboard('{ArrowDown}')
    expect(screen.getByRole('menuitemradio', { name: 'Apple' })).toHaveFocus()
    await user.keyboard('{Enter}')

    expect(changes).toEqual(['apple'])
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
    expect(trigger).toHaveFocus()

    await user.click(trigger)
    expect(screen.getByRole('menuitemradio', { name: 'Apple', checked: true })).toBeInTheDocument()
  })

  it('closes on Escape without changing the value, keeping focus on the trigger', async () => {
    const changes: Fruit[] = []
    const user = userEvent.setup()
    render(<Harness onChanged={(value) => changes.push(value)} />)

    await user.click(screen.getByRole('button', { name: 'Fruit' }))
    await user.keyboard('{ArrowUp}{Escape}')

    expect(changes).toEqual([])
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Fruit' })).toHaveFocus()
  })

  it('closes on Tab and continues to the adjacent page control', async () => {
    const user = userEvent.setup()
    render(
      <>
        <Harness />
        <button>After menu</button>
      </>,
    )

    await user.click(screen.getByRole('button', { name: 'Fruit' }))
    await user.tab()

    await waitFor(() => expect(screen.queryByRole('menu')).not.toBeInTheDocument())
    expect(screen.getByRole('button', { name: 'After menu' })).toHaveFocus()
  })

  it('closes when a pointer lands outside and returns focus to the trigger', async () => {
    const user = userEvent.setup()
    render(<Harness />)
    const trigger = screen.getByRole('button', { name: 'Fruit' })
    await user.click(trigger)
    expect(screen.getByRole('menu')).toBeInTheDocument()
    fireEvent.pointerDown(document.body)
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
    await waitFor(() => expect(trigger).toHaveFocus())
  })
})
