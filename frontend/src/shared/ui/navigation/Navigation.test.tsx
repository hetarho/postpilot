import { render, screen } from '@testing-library/react'
import { NavigationProvider } from './NavigationProvider'
import { ContextualReturn } from './ContextualReturn'

describe('injected navigation primitives', () => {
  it('leads back to a real contextual return before the structural parent', () => {
    render(
      <NavigationProvider
        value={{
          ancestors: [{ href: '/parent', label: 'Parent' }],
          returnTo: { href: '/history?status=failed', label: 'Return to filtered history' },
        }}
      >
        <ContextualReturn />
      </NavigationProvider>,
    )
    expect(screen.getAllByRole('link')).toHaveLength(1)
    expect(screen.getByRole('link', { name: 'Return to filtered history' })).toHaveAttribute(
      'href',
      '/history?status=failed',
    )
  })
  it('falls back to the nearest safe parent and does not render unsafe links', () => {
    render(
      <NavigationProvider
        value={{
          returnTo: { href: '//evil-return.test', label: 'Unsafe return' },
          ancestors: [
            { href: '/parent', label: 'Parent' },
            { href: '//evil.test', label: 'Unsafe' },
          ],
        }}
      >
        <ContextualReturn />
      </NavigationProvider>,
    )
    expect(screen.queryByRole('link', { name: 'Unsafe' })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Unsafe return' })).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Parent' })).toHaveAttribute('href', '/parent')
  })
  it('renders nothing for a root with no parent or return', () => {
    render(
      <NavigationProvider value={{ ancestors: [] }}>
        <ContextualReturn />
      </NavigationProvider>,
    )
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
  })
  it('supports an injected router/flush link and tolerates absent providers', () => {
    const { rerender } = render(<ContextualReturn />)
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
    rerender(
      <NavigationProvider
        value={{
          ancestors: [{ href: '/parent', label: 'Parent' }],
          Link: ({ children, ...props }) => (
            <a {...props} data-flush-guard="true">
              {children}
            </a>
          ),
        }}
      >
        <ContextualReturn />
      </NavigationProvider>,
    )
    expect(screen.getByRole('link')).toHaveAttribute('data-flush-guard', 'true')
  })
})
