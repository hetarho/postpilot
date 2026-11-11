import { screen, within } from '@testing-library/react'
import { expect, it } from 'vitest'
import { renderAppAt } from '@/test/app'

const hrefs = (scope: HTMLElement) =>
  within(scope)
    .getAllByRole('link')
    .map((link) => link.getAttribute('href'))

// THEME-70 →THEME-38: 내 스타일's overview lists its sections in the chip row's order, so the
// cards and the chips under a section's title read the hub the same way.
it('orders the overview cards as the hub chips are ordered', async () => {
  const overview = renderAppAt('/style', { user: { id: 'alice' } })
  const cards = hrefs(await screen.findByRole('navigation', { name: '알려준 것' }))
  overview.unmount()

  renderAppAt('/voices', { user: { id: 'alice' } })
  const chips = hrefs(await screen.findByRole('navigation', { name: '나누어 보기' }))
  expect(cards).toEqual(chips)
  expect(cards).toEqual(['/voices', '/templates', '/guidelines', '/memories', '/spoken-voices'])
})
