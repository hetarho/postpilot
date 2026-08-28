import { render, screen } from '@testing-library/react'
import { Badge } from './Badge'

describe('Badge', () => {
  it('renders a neutral inline status without adding interactive semantics', () => {
    render(<Badge>초안</Badge>)

    expect(screen.getByText('초안')).toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })
})
