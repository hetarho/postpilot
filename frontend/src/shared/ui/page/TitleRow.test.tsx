import { render, screen } from '@testing-library/react'
import { expect, it, vi } from 'vitest'
import { NavigationProvider } from '../navigation/NavigationProvider'
import { PageFrame } from './PageFrame'
import { TitleRow } from './TitleRow'

const withReturn = (claimReturn = vi.fn(() => () => undefined)) => ({
  ancestors: [{ href: '/', label: 'Home' }],
  hasReturn: true,
  claimReturn,
})

it('puts the way back at the start of the title row and claims it from the shell', () => {
  const claimReturn = vi.fn(() => () => undefined)
  render(
    <NavigationProvider value={withReturn(claimReturn)}>
      <TitleRow>
        <h1>Guidelines</h1>
      </TitleRow>
    </NavigationProvider>,
  )
  const back = screen.getByRole('link', { name: 'Home' })
  // The ← and the title share one row: the title's column is the arrow's next sibling.
  expect(back.parentElement?.nextElementSibling).toContainElement(
    screen.getByRole('heading', { name: 'Guidelines' }),
  )
  expect(claimReturn).toHaveBeenCalledTimes(1)
})

it('takes the way back from a frame, which then draws none of its own', () => {
  render(
    <NavigationProvider value={withReturn()}>
      <PageFrame>
        <TitleRow>
          <h1>Admin</h1>
        </TitleRow>
      </PageFrame>
    </NavigationProvider>,
  )
  expect(screen.getAllByRole('link', { name: 'Home' })).toHaveLength(1)
})

it('draws no way back where the route has none, and a screen’s own one where it passes it', () => {
  const { rerender } = render(
    <NavigationProvider value={{ ancestors: [] }}>
      <TitleRow>
        <h1>Home</h1>
      </TitleRow>
    </NavigationProvider>,
  )
  expect(screen.queryByRole('link')).toBeNull()
  rerender(
    <NavigationProvider value={{ ancestors: [] }}>
      <TitleRow back={<a href="/plans">Plans</a>}>
        <h1>Checkout</h1>
      </TitleRow>
    </NavigationProvider>,
  )
  expect(screen.getByRole('link', { name: 'Plans' })).toHaveAttribute('href', '/plans')
})
