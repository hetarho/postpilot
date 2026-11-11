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

  // Unticked, the box was only a recessed well: it vanished on Day's page and read as a bullet
  // in Night (출처 보기). It carries an edge until it is ticked and the fill takes over.
  it('draws an edge on the unticked box that the tick fill replaces', () => {
    render(<Checkbox aria-label="출처 보기" />)

    const box = screen.getByRole('checkbox', { name: '출처 보기' }).nextElementSibling
    expect(box).toHaveClass('ring-2', 'ring-inset', 'ring-checkbox-edge')
    expect(box).toHaveClass('peer-checked:ring-transparent', 'peer-checked:bg-button-cta-bg')
  })
})
