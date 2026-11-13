import { act, render, screen, waitFor } from '@testing-library/react'
import { LoadingState, LoadingStatus, Skeleton } from './LoadingState'

describe('LoadingState', () => {
  it.each(['list', 'cards', 'form', 'detail', 'table', 'inline'] as const)(
    'exposes one polite status and a busy region for %s',
    async (variant) => {
      const { container } = render(<LoadingState variant={variant} label="불러오는 중" />)
      const status = screen.getByRole('status')
      await waitFor(() => expect(status).toHaveTextContent('불러오는 중'))
      expect(status).toHaveAttribute('aria-live', 'polite')
      expect(status.closest('[aria-busy="true"]')).toBeNull()
      expect(container.querySelector('.loading-reveal')).toHaveAttribute('aria-busy', 'true')
      expect(screen.getAllByRole('status')).toHaveLength(1)
      expect(container.querySelectorAll('button, input, [tabindex]')).toHaveLength(0)
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

it('registers an empty loading status before announcing and keeps later updates on that node', () => {
  const frames: FrameRequestCallback[] = []
  vi.stubGlobal(
    'requestAnimationFrame',
    vi.fn((callback: FrameRequestCallback) => {
      frames.push(callback)
      return frames.length
    }),
  )
  vi.stubGlobal('cancelAnimationFrame', vi.fn())
  try {
    const { rerender, unmount } = render(<LoadingStatus label="Loading" />)
    const status = screen.getByRole('status')
    expect(status).toBeEmptyDOMElement()
    act(() => frames.shift()!(0))
    expect(status).toBeEmptyDOMElement()
    act(() => frames.shift()!(16))
    expect(status).toHaveTextContent('Loading')
    rerender(<LoadingStatus label="Failed" />)
    expect(screen.getByRole('status')).toBe(status)
    expect(status).toHaveTextContent('Failed')
    unmount()
  } finally {
    vi.unstubAllGlobals()
  }
})
