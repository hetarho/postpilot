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

// THEME-67: 새 글 / 새 영상 is the channel's creation entry, so the new post or clip comes back to
// the channel it was started from (THEME-58), and pressing it starts no paid work. Videos are one
// list: the old Instagram address lands on it, and a video started there returns to it.
it.each([
  ['/posts', '새 글', '/posts/new', '네이버 블로그로 돌아가기', '/posts'],
  ['/clips', '새 영상', '/clips/new', '영상으로 돌아가기', '/clips'],
] as const)(
  'starts creation from %s at once and keeps the list as its origin',
  async (path, label, creation, back, origin) => {
    const user = userEvent.setup()
    const calls: string[] = []
    const view = renderAppAt(path, { user: { id: 'alice' }, calls })
    await user.click(await screen.findByRole('link', { name: label }))
    await waitFor(() => expect(view.router.state.location.pathname).toBe(creation))
    expect(calls.filter((call) => /^(Start|Create|Generate|Analyze|Save)/.test(call))).toEqual([])
    expect(
      path === '/posts' ? readPostReturnContext('alice') : readClipReturnContext('alice'),
    ).toMatchObject({ path: origin })
    expect(await screen.findByRole('link', { name: back })).toHaveAttribute('href', origin)
  },
)

// Each section's button opens that section's creation directly, carrying no channel (an old link's is ignored).
it.each([
  ['/templates', '새 템플릿', '/templates/new'],
  ['/video-templates', '새 템플릿', '/video-templates/new'],
] as const)('offers %s its own creation only', async (path, label, href) => {
  renderAppAt(path, { user: { id: 'alice' } })
  expect(await screen.findByRole('link', { name: label })).toHaveAttribute('href', href)
  expect(screen.queryByRole('menu')).toBeNull()
  expect(document.querySelector('[data-action-bar="list"] [aria-haspopup]')).toBeNull()
})

// A section with several ways to make one keeps ONE creation button, and its ways open from it
// (a choice opens from the control that needs it); none of them reaches another section.
it.each([
  ['/voices', '새 말투 만들기', ['내가 쓴 글로 만들기', 'AI 말투 추천받기']],
  ['/guidelines', '새 지침', ['AI와 함께 만들기', '직접 쓰기', '기본 지침에서 고르기']],
  ['/video-guidelines', '새 지침', ['AI와 함께 만들기', '직접 쓰기', '기본 지침에서 고르기']],
] as const)('opens %s ways to make one from its one creation button', async (path, label, ways) => {
  const user = userEvent.setup()
  const calls: string[] = []
  renderAppAt(path, { user: { id: 'alice' }, calls })
  await user.click(await screen.findByRole('button', { name: label }))
  const menu = await screen.findByRole('menu', { name: label })
  expect(
    within(menu)
      .getAllByRole('menuitem')
      .map((item) => item.textContent),
  ).toEqual(ways.map((way) => expect.stringContaining(way)))
  expect(calls.filter((call) => /^(Start|Create|Generate|Analyze|Save)/.test(call))).toEqual([])
})

it.each(['/posts/new', '/clips/new'])(
  'keeps the creation button off the writing screen %s',
  async (path) => {
    renderAppAt(path, { user: { id: 'alice' } })
    // Guide: a writing screen is a focused flow — no tab bar, no section chips, no creation
    // button; only the account menu stays in the slim top bar.
    await screen.findByRole('button', { name: '내 계정' })
    await screen.findByRole('main')
    expect(screen.queryByRole('navigation', { name: '주요' })).toBeNull()
    expect(screen.queryByRole('link', { name: /^새 (글|영상)$/ })).toBeNull()
    expect(screen.queryByRole('navigation', { name: '채널 메뉴' })).toBeNull()
  },
)
