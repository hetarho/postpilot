import { createRef } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Checkbox } from './Checkbox'

describe('Checkbox', () => {
  it('keeps native checked, disabled, and ref semantics', async () => {
    const ref = createRef<HTMLInputElement>()
    const user = userEvent.setup()
    render(<Checkbox ref={ref} aria-label="규칙 저장" />)

    const checkbox = screen.getByRole('checkbox', { name: '규칙 저장' })
    await user.click(checkbox)

    expect(checkbox).toBeChecked()
    expect(ref.current).toBe(checkbox)
  })
})
