import { render, screen } from '@testing-library/react'
import { NavigationProvider } from './NavigationProvider'
import { Breadcrumb } from './Breadcrumb'
import { ContextualReturn } from './ContextualReturn'

describe('injected navigation primitives', () => {
  it('names the current location and preserves a real contextual return link', () => {
    render(
      <NavigationProvider
        value={{
          current: 'Current',
          ancestors: [{ href: '/parent', label: 'Parent' }],
          returnTo: { href: '/history?status=failed', label: 'Return to filtered history' },
        }}
      >
        <Breadcrumb ariaLabel="Location" />
        <ContextualReturn />
      </NavigationProvider>,
    )
    expect(screen.getByRole('navigation', { name: 'Location' })).toBeInTheDocument()
    expect(screen.getByText('Current')).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('link', { name: 'Parent' })).toHaveAttribute('href', '/parent')
    expect(screen.getByRole('link', { name: 'Return to filtered history' })).toHaveAttribute(
      'href',
      '/history?status=failed',
    )
  })
  it('falls back to the named parent and does not render unsafe links', () => {
    render(
      <NavigationProvider
        value={{
          current: 'Current',
          returnTo: { href: '//evil-return.test', label: 'Unsafe return' },
          ancestors: [
            { href: '//evil.test', label: 'Unsafe' },
            { href: '/parent', label: 'Parent' },
          ],
        }}
      >
        <Breadcrumb ariaLabel="Location" />
        <ContextualReturn />
      </NavigationProvider>,
    )
    expect(screen.queryByRole('link', { name: 'Unsafe' })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Unsafe return' })).not.toBeInTheDocument()
    expect(screen.getAllByRole('link', { name: 'Parent' })).toHaveLength(2)
  })
  it('supports an injected router/flush link and tolerates absent providers', () => {
    const { rerender } = render(
      <>
        <Breadcrumb ariaLabel="Location" />
        <ContextualReturn />
      </>,
    )
    expect(screen.queryByRole('navigation')).not.toBeInTheDocument()
    rerender(
      <NavigationProvider
        value={{
          current: 'Here',
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
