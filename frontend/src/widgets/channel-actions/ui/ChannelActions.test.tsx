import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { cleanup, screen, waitFor } from '@testing-library/react'
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

// THEME-67: 새 글 / 새 영상 is the channel's creation entry, so the new post or clip comes back to
// the channel it was started from (THEME-58), and pressing it starts no paid work.
it.each([
  ['/naver-blog', '새 글', '/posts/new', '네이버 블로그로 돌아가기'],
  ['/naver-clip', '새 영상', '/clips/new', '네이버 클립으로 돌아가기'],
  ['/insta-clip', '새 영상', '/clips/new', '인스타 클립으로 돌아가기'],
] as const)(
  'starts creation from %s at once and keeps the channel as its origin',
  async (path, label, creation, back) => {
    const user = userEvent.setup()
    const calls: string[] = []
    const view = renderAppAt(path, { user: { id: 'alice' }, calls })
    await user.click(await screen.findByRole('link', { name: label }))
    await waitFor(() => expect(view.router.state.location.pathname).toBe(creation))
    expect(calls.filter((call) => /^(Start|Create|Generate|Analyze|Save)/.test(call))).toEqual([])
    expect(
      path === '/naver-blog' ? readPostReturnContext('alice') : readClipReturnContext('alice'),
    ).toMatchObject({ path })
    expect(await screen.findByRole('link', { name: back })).toHaveAttribute('href', path)
  },
)

// Each section's button opens that section's creation directly, keeping a clip channel.
it.each([
  ['/templates', '새 템플릿', '/templates/new'],
  ['/voices', '새 말투', '/voices?new=1'],
  ['/guidelines', '새 지침', '/guidelines?new=1'],
  ['/voices?channel=instagram_clip', '새 말투', '/voices?new=1&channel=instagram_clip'],
  ['/video-guidelines?channel=naver_clip', '새 지침', '/video-guidelines?new=1&channel=naver_clip'],
  [
    '/video-templates?channel=instagram_clip',
    '새 템플릿',
    '/video-templates/new?channel=instagram_clip',
  ],
] as const)('offers %s its own creation only', async (path, label, href) => {
  renderAppAt(path, { user: { id: 'alice' } })
  expect(await screen.findByRole('link', { name: label })).toHaveAttribute('href', href)
  expect(screen.queryByRole('menu')).toBeNull()
  expect(document.querySelector('[data-action-bar="list"] [aria-haspopup]')).toBeNull()
})

it.each(['/posts/new', '/clips/new?channel=naver_clip'])(
  'keeps the creation button off the writing screen %s',
  async (path) => {
    renderAppAt(path, { user: { id: 'alice' } })
    await screen.findByRole('navigation', { name: '주요' })
    expect(screen.queryByRole('link', { name: /^새 (글|영상)$/ })).toBeNull()
    expect(screen.queryByRole('navigation', { name: '채널 메뉴' })).toBeNull()
  },
)
