import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { readClipReturnContext } from '@/entities/clip-project'
import { readPostReturnContext } from '@/entities/post'
import { renderAppAt } from '@/test/app'
beforeEach(() => initializeI18n('ko'))
afterEach(() => {
  cleanup()
  sessionStorage.clear()
  vi.unstubAllGlobals()
})

// THEME-67: opening the menu is free; 새 글 / 새 영상 is the channel's creation entry, so the new
// post or clip comes back to the channel it was started from (THEME-58).
it.each([
  ['/naver-blog', '새 글', '새 글쓰기', '/posts/new', '네이버 블로그로 돌아가기'],
  ['/naver-clip', '새 영상', '새 영상 만들기', '/clips/new', '네이버 클립으로 돌아가기'],
  ['/insta-clip', '새 영상', '새 영상 만들기', '/clips/new', '인스타 클립으로 돌아가기'],
] as const)(
  'starts creation from %s and keeps the channel as its origin',
  async (path, trigger, action, creation, back) => {
    const user = userEvent.setup()
    const calls: string[] = []
    const view = renderAppAt(path, { user: { id: 'alice' }, calls })
    await user.click(await screen.findByRole('button', { name: trigger }))
    expect(calls.filter((call) => /^(Start|Create|Generate|Analyze|Save)/.test(call))).toEqual([])
    await user.click(screen.getByRole('menuitem', { name: action }))
    await waitFor(() => expect(view.router.state.location.pathname).toBe(creation))
    expect(
      path === '/naver-blog' ? readPostReturnContext('alice') : readClipReturnContext('alice'),
    ).toMatchObject({ path })
    expect(await screen.findByRole('link', { name: back })).toHaveAttribute('href', path)
  },
)

// The current section's creation stands alone nearest the button; the other three keep the
// section order above it.
it.each([
  ['/voices', '새 말투', ['새 글쓰기', '새 템플릿', '새 지침'], '/voices?new=1'],
  [
    '/video-guidelines?channel=naver_clip',
    '새 지침',
    ['새 영상 만들기', '새 템플릿', '새 말투'],
    '/video-guidelines?new=1&channel=naver_clip',
  ],
  [
    '/video-templates?channel=instagram_clip',
    '새 템플릿',
    ['새 영상 만들기', '새 말투', '새 지침'],
    '/video-templates/new?channel=instagram_clip',
  ],
] as const)(
  'orders the menu on %s with its own creation last',
  async (path, label, others, href) => {
    const user = userEvent.setup()
    renderAppAt(path, { user: { id: 'alice' } })
    await user.click(await screen.findByRole('button', { name: label }))
    const [above, nearest] = within(screen.getByRole('menu')).getAllByRole('group')
    expect(
      within(above)
        .getAllByRole('menuitem')
        .map((item) => item.textContent),
    ).toEqual(others)
    expect(within(nearest).getByRole('menuitem', { name: label })).toHaveAttribute('href', href)
  },
)

it.each(['/posts/new', '/clips/new?channel=naver_clip'])(
  'keeps the creation menu off the writing screen %s',
  async (path) => {
    renderAppAt(path, { user: { id: 'alice' } })
    await screen.findByRole('navigation', { name: '주요' })
    await waitFor(() =>
      expect(document.querySelector('[data-action-bar="list"] [aria-haspopup="menu"]')).toBeNull(),
    )
    expect(screen.queryByRole('navigation', { name: '채널 메뉴' })).toBeNull()
  },
)
