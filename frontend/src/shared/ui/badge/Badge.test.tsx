import { render, screen } from '@testing-library/react'
import { Badge } from './Badge'

describe('Badge', () => {
  it('renders a neutral inline status without adding interactive semantics', () => {
    render(<Badge>초안</Badge>)

    expect(screen.getByText('초안')).toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  // The owner read 초안 · 검토 on a post card as too small: it was 12px beside 13px metadata.
  it('sets its label a step above the metadata beside it', () => {
    render(<Badge tone="accent">검토</Badge>)

    expect(screen.getByText('검토')).toHaveClass('text-sm', 'font-semibold')
    expect(screen.getByText('검토')).not.toHaveClass('text-xs')
  })
})
