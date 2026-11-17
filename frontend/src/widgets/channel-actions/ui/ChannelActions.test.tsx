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

// A section that still docks its creation opens it directly, carrying no channel. The writing
// kit's screens and 영상 지침 make theirs from their heading row instead.
it.each([['/video-templates', '새 템플릿', '/video-templates/new']] as const)(
  'offers %s its own creation only',
  async (path, label, href) => {
    renderAppAt(path, { user: { id: 'alice' } })
    expect(await screen.findByRole('link', { name: label })).toHaveAttribute('href', href)
    expect(screen.queryByRole('menu')).toBeNull()
    expect(document.querySelector('[data-action-bar="list"] [aria-haspopup]')).toBeNull()
  },
)

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
