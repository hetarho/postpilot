import { createRef } from 'react'
import { render, screen } from '@testing-library/react'
import { Select } from './Select'

describe('Select', () => {
  it('keeps native options, disabled state, and ref semantics', () => {
    const ref = createRef<HTMLSelectElement>()
    render(
      <Select ref={ref} aria-label="모델" disabled defaultValue="one">
        <option value="one">하나</option>
        <option value="two" disabled>
          둘
        </option>
      </Select>,
    )

    const select = screen.getByRole('combobox', { name: '모델' })
    expect(select).toBeDisabled()
    expect(select).toHaveValue('one')
    expect(ref.current).toBe(select)
    expect(screen.getByRole('option', { name: '둘' })).toBeDisabled()
  })

  it('keeps placeholder and invalid semantics native', () => {
    render(
      <Select aria-label="모델" aria-invalid defaultValue="">
        <option value="">모델 선택</option>
        <option value="one">하나</option>
      </Select>,
    )

    const select = screen.getByRole('combobox', { name: '모델' })
    expect(select).toHaveValue('')
    expect(select).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByRole('option', { name: '모델 선택' })).toBeInTheDocument()
  })
})
