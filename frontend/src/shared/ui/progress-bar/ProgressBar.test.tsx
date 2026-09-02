import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { ProgressBar } from './ProgressBar'

describe('ProgressBar', () => {
  it('reports a determinate ratio through its bounds and its fill', () => {
    render(<ProgressBar label="작업 진행률" done={3} total={8} />)
    const bar = screen.getByRole('progressbar', { name: '작업 진행률' })
    expect(bar).toHaveAttribute('aria-valuenow', '3')
    expect(bar).toHaveAttribute('aria-valuemin', '0')
    expect(bar).toHaveAttribute('aria-valuemax', '8')
    expect(bar.firstElementChild).toHaveStyle({ inlineSize: '37.5%' })
  })

  it('omits aria-valuenow entirely where the stage reports no ratio', () => {
    render(<ProgressBar label="작업 진행률" />)
    const bar = screen.getByRole('progressbar', { name: '작업 진행률' })
    expect(bar).not.toHaveAttribute('aria-valuenow')
    expect(bar.firstElementChild).toHaveClass('animate-progress-sweep')
  })

  // A total of 0 is what a stage with nothing to count reports, and 0/0 is not 'complete'.
  it('treats an empty total as indeterminate rather than as full', () => {
    render(<ProgressBar label="작업 진행률" done={0} total={0} />)
    expect(screen.getByRole('progressbar')).not.toHaveAttribute('aria-valuenow')
  })

  // A job projection that over- or under-reports must not announce a value outside the range it
  // is announced against.
  it('bounds the announced value by its own maximum', () => {
    const { rerender } = render(<ProgressBar label="작업 진행률" done={9} total={8} />)
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '8')
    rerender(<ProgressBar label="작업 진행률" done={-2} total={8} />)
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '0')
  })

  it('takes no layout height, so it cannot move the page it is pinned to', () => {
    const { container } = render(<ProgressBar label="작업 진행률" done={1} total={2} />)
    expect(container.firstElementChild).toHaveClass('h-0')
    expect(screen.getByRole('progressbar')).toHaveClass('absolute', 'h-progress-bar')
  })
})
