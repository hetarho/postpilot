import { useState } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it } from 'vitest'
import { RadioGroup } from './RadioGroup'

it('lets a keyboard listener select a candidate while skipping unavailable choices', async () => {
  function Candidates() {
    const [value, setValue] = useState('one')
    return (
      <RadioGroup
        label="Candidates"
        value={value}
        onChange={setValue}
        options={[
          { value: 'one', label: 'One' },
          { value: 'two', label: 'Two', disabled: true },
          { value: 'three', label: 'Three' },
        ]}
      />
    )
  }
  const user = userEvent.setup()
  render(<Candidates />)
  await user.tab()
  expect(screen.getByRole('radio', { name: 'One' })).toHaveFocus()
  await user.keyboard('{ArrowDown}')
  expect(screen.getByRole('radio', { name: 'Three' })).toBeChecked()
  expect(screen.getByRole('radio', { name: 'Three' })).toHaveFocus()
  await user.keyboard('{ArrowDown}')
  expect(screen.getByRole('radio', { name: 'One' })).toBeChecked()
  await user.keyboard('{ArrowUp}')
  expect(screen.getByRole('radio', { name: 'Three' })).toBeChecked()
})
