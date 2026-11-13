import { render, screen } from '@testing-library/react'
import { LoadingState, Skeleton } from './LoadingState'

describe('LoadingState', () => {
  it.each(['list', 'cards', 'form', 'detail', 'table', 'inline'] as const)(
    'exposes one polite status and a busy region for %s',
    (variant) => {
      const { container } = render(<LoadingState variant={variant} label="불러오는 중" />)
      const status = screen.getByRole('status')
      expect(status).toHaveTextContent('불러오는 중')
      expect(status).toHaveAttribute('aria-live', 'polite')
      expect(status.parentElement).toHaveAttribute('aria-busy', 'true')
      expect(screen.getAllByRole('status')).toHaveLength(1)
      expect(container.querySelectorAll('button, input, [tabindex]')).toHaveLength(0)
      expect(container.querySelector('.loading-reveal')).toHaveAttribute('aria-hidden', 'true')
      if (variant === 'inline') expect(container.querySelector('.animate-spin')).toBeInTheDocument()
      else expect(container.querySelectorAll('.loading-skeleton').length).toBeGreaterThan(0)
    },
  )

  it('bounds representative rows and preserves the caller class', () => {
    const { container, rerender } = render(
      <LoadingState variant="form" label="Loading" rows={100} className="my-loading" />,
    )
    expect(container.firstChild).toHaveClass('my-loading')
    expect(container.querySelectorAll('.loading-skeleton')).toHaveLength(12)
    rerender(<LoadingState variant="form" label="Loading" rows={-1} />)
    expect(container.querySelectorAll('.loading-skeleton')).toHaveLength(2)
    rerender(<LoadingState variant="form" label="Loading" rows={Number.NaN} />)
    expect(container.querySelectorAll('.loading-skeleton')).toHaveLength(6)
  })

  it('keeps a standalone skeleton decorative and nonfocusable', () => {
    const { container } = render(<Skeleton className="h-4" />)
    expect(container.firstChild).toHaveAttribute('aria-hidden', 'true')
    expect(container.firstChild).toHaveClass('h-4')
    expect(container.firstChild).not.toHaveAttribute('tabindex')
  })
})
