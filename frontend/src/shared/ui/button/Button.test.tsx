import { createRef } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Button } from './Button'
import { buttonStyles } from './buttonStyles'

describe('Button', () => {
  it('is a non-submitting button by default and forwards its ref', () => {
    const ref = createRef<HTMLButtonElement>()
    render(<Button ref={ref}>동작</Button>)

    expect(screen.getByRole('button', { name: '동작' })).toHaveAttribute('type', 'button')
    expect(ref.current).toBe(screen.getByRole('button', { name: '동작' }))
  })

  it('keeps native submit, disabled, and click semantics', async () => {
    const user = userEvent.setup()
    const onClick = vi.fn()
    render(
      <Button type="submit" disabled onClick={onClick}>
        저장
      </Button>,
    )

    const button = screen.getByRole('button', { name: '저장' })
    expect(button).toBeDisabled()
    expect(button).toHaveAttribute('type', 'submit')
    await user.click(button)
    expect(onClick).not.toHaveBeenCalled()
  })

  it.each([
    ['cta', 'bg-button-cta-bg'],
    ['secondary', 'bg-button-secondary-bg'],
    ['ghost', 'bg-button-ghost-bg'],
    ['danger', 'text-button-danger-quiet-fg'],
  ] as const)('gives the production-backed %s variant its functional role', (variant, role) => {
    expect(buttonStyles({ variant })).toContain(role)
    expect(buttonStyles({ variant })).toContain('min-h-11')
  })

  it('keeps icon actions at least 44px square', () => {
    const styles = buttonStyles({ variant: 'danger', size: 'icon' })
    expect(styles).toContain('size-11')
    expect(styles).toContain('shrink-0')
  })
})
