import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { NavigationProvider } from './NavigationProvider'
import { Breadcrumb } from './Breadcrumb'
import { ContextualReturn } from './ContextualReturn'

describe('injected navigation primitives', () => {
  it('discloses every ancestor and a distinct contextual return from the compact current location', async () => {
    const user = userEvent.setup()
    render(
      <NavigationProvider
        value={{
          current: 'Current question',
          ancestors: [
            { href: '/settings', label: 'Settings' },
            { href: '/settings#writing', label: 'Writing' },
            { href: '/voices', label: 'Voices' },
          ],
          returnTo: { href: '/posts/draft?filter=mine', label: 'Return to my draft' },
          Link: ({ children, ...props }) => (
            <a {...props} data-flush-guard="true">
              {children}
            </a>
          ),
        }}
      >
        <Breadcrumb ariaLabel="Location" compact />
      </NavigationProvider>,
    )
    const trigger = screen.getByRole('button', { name: 'Location: Current question' })
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
    await user.click(trigger)
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByRole('dialog', { name: 'Location: Current question' })).toBeInTheDocument()
    expect(screen.getAllByRole('link').map((link) => link.getAttribute('href'))).toEqual([
      '/settings',
      '/settings#writing',
      '/voices',
      '/posts/draft?filter=mine',
    ])
    for (const link of screen.getAllByRole('link'))
      expect(link).toHaveAttribute('data-flush-guard', 'true')
    expect(screen.getByRole('link', { name: 'Settings' })).toHaveFocus()
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(trigger).toHaveFocus()
  })
  it('keeps a compact root visible without inventing a parent or accepting an unsafe return', () => {
    render(
      <NavigationProvider
        value={{
          current: 'Writing tests',
          ancestors: [{ href: '//evil.test', label: 'Unsafe parent' }],
          returnTo: { href: 'https://evil.test', label: 'Unsafe return' },
        }}
      >
        <Breadcrumb ariaLabel="Location" compact />
      </NavigationProvider>,
    )
    expect(screen.getByRole('navigation', { name: 'Location' })).toHaveTextContent('Writing tests')
    expect(screen.getByText('Writing tests')).toHaveAttribute('aria-current', 'page')
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
  })
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
