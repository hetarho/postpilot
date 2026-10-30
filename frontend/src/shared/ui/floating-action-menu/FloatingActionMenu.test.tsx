import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { FileText, ListChecks, PenLine } from 'lucide-react'
import { NavigationProvider } from '../navigation/NavigationProvider'
import type { NavigationLinkProps } from '../navigation/context'
import { FloatingActionMenu, type FloatingActionItem } from './FloatingActionMenu'

function groups(onCreate = vi.fn()): FloatingActionItem[][] {
  return [
    [
      { id: 'templates', label: '글 템플릿', icon: FileText, href: '/templates' },
      { id: 'guidelines', label: '글 지침', icon: ListChecks, href: '/guidelines' },
    ],
    [{ id: 'create', label: '새 글 쓰기', icon: PenLine, href: '/posts/new', onSelect: onCreate }],
  ]
}

function renderMenu(onCreate = vi.fn()) {
  render(
    <FloatingActionMenu
      label="새 글"
      closeLabel="닫기"
      menuLabel="블로그 메뉴"
      groups={groups(onCreate)}
    />,
  )
  return screen.getByRole('button', { name: '새 글' })
}

describe('FloatingActionMenu', () => {
  it('rests as one extended button in the list dock with the menu closed', () => {
    const trigger = renderMenu()
    expect(trigger).toHaveAttribute('aria-haspopup', 'menu')
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    expect(trigger.closest('[data-action-bar="list"]')).not.toBeNull()
    expect(screen.queryByRole('menu')).toBeNull()
    expect(screen.queryByTestId('floating-action-scrim')).toBeNull()
  })

  it('opens the groups above the button in order and focuses the first item', async () => {
    const user = userEvent.setup()
    await user.click(renderMenu())
    const close = screen.getByRole('button', { name: '닫기' })
    expect(close).toHaveAttribute('aria-expanded', 'true')
    const menu = screen.getByRole('menu', { name: '블로그 메뉴' })
    const [settings, creation] = within(menu).getAllByRole('group')
    expect(
      within(settings)
        .getAllByRole('menuitem')
        .map((item) => item.textContent),
    ).toEqual(['글 템플릿', '글 지침'])
    expect(within(creation).getByRole('menuitem')).toHaveAttribute('href', '/posts/new')
    expect(screen.getByTestId('floating-action-scrim')).toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('menuitem', { name: '글 템플릿' })).toHaveFocus())
  })

  it('closes on ×, scrim and Escape, returning focus to the button', async () => {
    const user = userEvent.setup()
    const trigger = renderMenu()

    await user.click(trigger)
    await user.click(screen.getByRole('button', { name: '닫기' }))
    expect(screen.queryByRole('menu')).toBeNull()
    await waitFor(() => expect(screen.getByRole('button', { name: '새 글' })).toHaveFocus())

    await user.click(screen.getByRole('button', { name: '새 글' }))
    await user.click(screen.getByTestId('floating-action-scrim'))
    expect(screen.queryByRole('menu')).toBeNull()
    await waitFor(() => expect(screen.getByRole('button', { name: '새 글' })).toHaveFocus())

    await user.click(screen.getByRole('button', { name: '새 글' }))
    await waitFor(() => expect(screen.getByRole('menuitem', { name: '글 템플릿' })).toHaveFocus())
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('menu')).toBeNull()
    await waitFor(() => expect(screen.getByRole('button', { name: '새 글' })).toHaveFocus())
  })

  it('moves across the groups with the arrow keys and jumps with Home/End', async () => {
    const user = userEvent.setup()
    renderMenu().focus()
    await user.keyboard('{ArrowUp}')
    await waitFor(() => expect(screen.getByRole('menuitem', { name: '글 템플릿' })).toHaveFocus())
    await user.keyboard('{ArrowDown}{ArrowDown}')
    expect(screen.getByRole('menuitem', { name: '새 글 쓰기' })).toHaveFocus()
    await user.keyboard('{ArrowDown}')
    expect(screen.getByRole('menuitem', { name: '글 템플릿' })).toHaveFocus()
    await user.keyboard('{End}')
    expect(screen.getByRole('menuitem', { name: '새 글 쓰기' })).toHaveFocus()
    await user.keyboard('{Home}')
    expect(screen.getByRole('menuitem', { name: '글 템플릿' })).toHaveFocus()
  })

  it('runs the item’s onSelect, then hands the click to the navigation link and closes', async () => {
    const user = userEvent.setup()
    const onCreate = vi.fn()
    const navigated: string[] = []
    function RouterLink({ href, onClick, ...props }: NavigationLinkProps) {
      return (
        <a
          {...props}
          href={href}
          onClick={(event) => {
            onClick?.(event)
            event.preventDefault()
            navigated.push(href)
          }}
        />
      )
    }
    render(
      <NavigationProvider value={{ ancestors: [], Link: RouterLink }}>
        <FloatingActionMenu
          label="새 글"
          closeLabel="닫기"
          menuLabel="블로그 메뉴"
          groups={groups(onCreate)}
        />
      </NavigationProvider>,
    )
    await user.click(screen.getByRole('button', { name: '새 글' }))
    await user.click(screen.getByRole('menuitem', { name: '새 글 쓰기' }))
    expect(onCreate).toHaveBeenCalledOnce()
    expect(navigated).toEqual(['/posts/new'])
    expect(screen.queryByRole('menu')).toBeNull()
  })
})
