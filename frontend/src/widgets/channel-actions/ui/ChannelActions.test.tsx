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

// THEME-67: opening the menu is free; its creation item is the channel's creation entry, so the
// new post or clip comes back to the channel it was started from (THEME-58).
it.each([
  ['/naver-blog', '새 글', '새 글 쓰기', '/posts/new', '네이버 블로그로 돌아가기'],
  ['/naver-clip', '새 클립', '새 클립 만들기', '/clips/new', '네이버 클립으로 돌아가기'],
  ['/insta-clip', '새 클립', '새 클립 만들기', '/clips/new', '인스타 클립으로 돌아가기'],
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

it('opens 말투 with the channel as its contextual return', async () => {
  const user = userEvent.setup()
  const view = renderAppAt('/insta-clip', { user: { id: 'alice' } })
  await user.click(await screen.findByRole('button', { name: '새 클립' }))
  await user.click(screen.getByRole('menuitem', { name: '말투' }))
  await waitFor(() => expect(view.router.state.location.pathname).toBe('/voices'))
  expect(view.router.state.location.search).toMatchObject({ entry: '/insta-clip' })
  const back = within(await screen.findByRole('navigation', { name: '돌아가기' })).getByRole('link')
  expect(back).toHaveAccessibleName('인스타 클립')
  expect(back).toHaveAttribute('href', '/insta-clip')
})
