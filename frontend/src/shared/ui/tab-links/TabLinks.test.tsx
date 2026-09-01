import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from '@tanstack/react-router'
import { History, IdCard } from 'lucide-react'
import { TabLinks, type TabLink } from './TabLinks'

function renderTabs(items: readonly TabLink[], at: string) {
  const root = createRootRoute({
    component: () => (
      <>
        <TabLinks items={items} ariaLabel="테스트 탭" />
        <Outlet />
      </>
    ),
  })
  const routeTree = root.addChildren(
    items.map((item) =>
      createRoute({ getParentRoute: () => root, path: item.to, component: () => null }),
    ),
  )
  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: [at] }),
  })
  // The generic parameter is the app's registered router; this throwaway tree is looser.
  return render(<RouterProvider router={router as never} />)
}

afterEach(cleanup)

describe('TabLinks', () => {
  it('keeps the scrolling text row when items carry no icons', async () => {
    renderTabs(
      [
        { to: '/profile', label: '프로필' },
        { to: '/versions', label: '버전 기록' },
      ],
      '/versions',
    )

    const current = await screen.findByRole('link', { name: '버전 기록' })
    expect(current).toHaveAttribute('aria-current', 'page')
    expect(current).toHaveClass('shrink-0', 'basis-auto', 'px-4')
    expect(current.querySelector('svg')).toBeNull()
    expect(screen.getByRole('navigation', { name: '테스트 탭' })).toHaveClass(
      'overflow-x-auto',
      'overscroll-x-contain',
    )
  })

  it('compacts to icon-over-caption tabs below the container breakpoint when every item has an icon', async () => {
    renderTabs(
      [
        { to: '/profile', label: '프로필', shortLabel: '프로필', icon: IdCard },
        { to: '/versions', label: '버전 기록', shortLabel: '버전', icon: History },
      ],
      '/profile',
    )

    // The full label stays the ONE accessible name in both shapes.
    const current = await screen.findByRole('link', { name: '프로필' })
    const sibling = screen.getByRole('link', { name: '버전 기록' })
    expect(current).toHaveAttribute('aria-current', 'page')

    // Compact is the unprefixed phone shape; the text row returns only when all English labels fit.
    expect(screen.getByRole('navigation', { name: '테스트 탭' })).toHaveClass('@container')
    expect(sibling).toHaveClass('flex-col', 'min-w-0', '@tabs:flex-row', '@tabs:px-1')
    expect(sibling.querySelector('svg')).toHaveClass('size-5', '@tabs:hidden')

    const [compactCaption, wideCaption] = Array.from(sibling.querySelectorAll('span'))
    expect(compactCaption).toHaveTextContent('버전')
    expect(compactCaption).toHaveClass('text-xs', '@tabs:hidden')
    expect(compactCaption).toHaveAttribute('aria-hidden', 'true')
    expect(wideCaption).toHaveTextContent('버전 기록')
    expect(wideCaption).toHaveClass('hidden', '@tabs:inline')
  })
})
