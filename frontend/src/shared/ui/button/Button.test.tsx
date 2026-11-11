import { createRef } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Button } from './Button'
import { buttonStyles } from './buttonStyles'
import { cardStyles } from '../card/cardStyles'

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
    ['secondary', 'bg-button-secondary-bg'],
    ['ghost', 'bg-button-ghost-bg'],
    ['danger', 'text-button-danger-quiet-fg'],
  ] as const)('gives the production-backed %s variant its functional role', (variant, role) => {
    expect(buttonStyles({ variant })).toContain(role)
    // The pointer floor (THEME-23): 40px under a mouse, the 44px touch floor under a thumb.
    expect(buttonStyles({ variant })).toContain('min-h-10')
    expect(buttonStyles({ variant })).toContain('pointer-coarse:min-h-11')
  })

  // Guide: a committing action is at least 48px at every pointer, and the step's one big action
  // takes the 56px hero size.
  it('gives the committing cta 48px and the hero action 56px at every pointer', () => {
    expect(buttonStyles({ variant: 'cta' })).toContain('bg-button-cta-bg')
    expect(buttonStyles({ variant: 'cta' })).toContain('min-h-12')
    expect(buttonStyles({ variant: 'cta', size: 'hero' })).toContain('min-h-14')
    expect(buttonStyles({ variant: 'cta' })).toContain('rounded-full')
  })

  it('keeps icon actions square at the pointer floor', () => {
    const styles = buttonStyles({ variant: 'danger', size: 'icon' })
    expect(styles).toContain('size-10')
    expect(styles).toContain('pointer-coarse:size-11')
    expect(styles).toContain('shrink-0')
    expect(styles).toContain('ui-button-icon')
  })

  // The one documented step below the floor, and it keeps a touch step of its own.
  it('gives compact the 32px desk height and 36px under a thumb', () => {
    const styles = buttonStyles({ size: 'compact' })
    expect(styles).toContain('min-h-8')
    expect(styles).toContain('pointer-coarse:min-h-9')
  })

  // A secondary pill is the card's own plane; on a card the stylesheet gives it the card's well
  // through these two hooks, so 기억으로 저장 on 더 해 보기 reads as a button, not loose text.
  it('marks a secondary button and a card for the on-card plane', () => {
    render(
      <section className={cardStyles()}>
        <Button>기억으로 저장</Button>
        <Button variant="ghost">닫기</Button>
      </section>,
    )
    expect(screen.getByRole('button', { name: '기억으로 저장' })).toHaveClass('ui-button-secondary')
    expect(screen.getByRole('button', { name: '닫기' })).not.toHaveClass('ui-button-secondary')
    expect(screen.getByRole('button', { name: '기억으로 저장' }).parentElement).toHaveClass(
      'ui-card',
    )
  })

  it('marks styled links and file labels for the same contextual dock sizing as buttons', () => {
    render(
      <>
        <Button variant="cta">생성</Button>
        <a href="/tests" className={buttonStyles({ variant: 'cta' })}>
          글쓰기 테스트
        </a>
        <label htmlFor="attachment" className={buttonStyles({ size: 'icon' })}>
          첨부
        </label>
        <input id="attachment" type="file" />
      </>,
    )

    expect(screen.getByRole('button', { name: '생성' })).toHaveClass('ui-button', 'ui-button-cta')
    expect(screen.getByRole('link', { name: '글쓰기 테스트' })).toHaveClass(
      'ui-button',
      'ui-button-cta',
    )
    expect(screen.getByText('첨부')).toHaveClass('ui-button', 'ui-button-icon')
    expect(screen.getByLabelText('첨부')).toHaveAttribute('type', 'file')
  })
})
