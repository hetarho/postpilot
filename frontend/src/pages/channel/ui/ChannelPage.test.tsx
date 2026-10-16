import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { cleanup, screen, within, waitFor } from '@testing-library/react'
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
it.each([
  ['/naver-blog', '네이버 블로그', ['/posts/new', '/posts', '/templates', '/guidelines']],
  [
    '/naver-clip',
    '네이버 클립',
    [
      '/clips/new?channel=naver_clip',
      '/clips?channel=naver_clip',
      '/video-templates',
      '/video-guidelines',
    ],
  ],
  [
    '/insta-clip',
    '인스타 클립',
    [
      '/clips/new?channel=instagram_clip',
      '/clips?channel=instagram_clip',
      '/video-templates',
      '/video-guidelines',
    ],
  ],
] as const)(
  'offers creation before history, templates and guidelines on %s without paid work',
  async (path, label, expected) => {
    const calls: string[] = []
    renderAppAt(path, { user: { id: 'alice' }, calls })
    const heading = await screen.findByRole('heading', { level: 1, name: label })
    const main = heading.closest('main')!
    expect(
      within(main)
        .getAllByRole('link')
        .map((link) => link.getAttribute('href')),
    ).toEqual(expected)
    expect(within(main).getByRole('navigation', { name: label })).toHaveTextContent('기록')
    expect(calls.filter((call) => /^(Start|Create|Generate|Analyze|Save)/.test(call))).toEqual([])
  },
)
it.each([
  ['/naver-blog', '네이버 블로그', '새 글 쓰기', '/naver-blog'],
  ['/naver-clip', '네이버 클립', '새 클립 만들기', '/naver-clip'],
  ['/insta-clip', '인스타 클립', '새 클립 만들기', '/insta-clip'],
] as const)('retains the channel creation origin from %s', async (path, label, action, parent) => {
  const view = renderAppAt(path, { user: { id: 'alice' } })
  await screen.findByRole('heading', { name: label, level: 1 })
  await userEvent.click(within(screen.getByRole('main')).getByRole('link', { name: action }))
  await waitFor(() =>
    expect(view.router.state.location.pathname).toBe(
      path === '/naver-blog' ? '/posts/new' : '/clips/new',
    ),
  )
  expect(
    path === '/naver-blog' ? readPostReturnContext('alice') : readClipReturnContext('alice'),
  ).toMatchObject({ path: parent })
  expect(
    await screen.findByRole('link', {
      name: `${label}${path === '/naver-blog' ? '로' : '으로'} 돌아가기`,
    }),
  ).toHaveAttribute('href', parent)
})
