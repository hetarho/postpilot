import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { cleanup, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { readClipReturnContext } from '@/entities/clip-project'
import { renderAppAt } from '@/test/app'
import { createThemeTestEnvironment } from '@/test/theme'

beforeEach(() => initializeI18n('ko'))
afterEach(() => {
  cleanup()
  localStorage.clear()
  sessionStorage.clear()
  initializeI18n('ko')
})
describe('channel launch', () => {
  it.each(['light', 'dark'] as const)(
    'offers one entry per channel in the %s theme without creating work',
    async (storedPreference) => {
      const calls: string[] = []
      const theme = createThemeTestEnvironment({ storedPreference })
      renderAppAt('/', { user: { id: 'home-alice' }, calls, theme: theme.ports })
      const choices = await screen.findByRole('navigation', { name: '채널 선택' })
      const links = within(choices).getAllByRole('link')
      expect(links).toHaveLength(3)
      expect(links.map((link) => link.getAttribute('href'))).toEqual([
        '/posts/new',
        '/clips/new?channel=naver_clip',
        '/clips/new?channel=instagram_clip',
      ])
      expect(links.map((link) => link.getAttribute('aria-labelledby'))).toEqual([
        'home-naverBlog-label',
        'home-naverClip-label',
        'home-instaClip-label',
      ])
      expect(links[0]).toHaveAccessibleName('네이버 블로그')
      expect(links[1]).toHaveAccessibleName('네이버 클립')
      expect(links[2]).toHaveAccessibleName('인스타 클립')
      expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1)
      expect(screen.queryByRole('combobox')).toBeNull()
      expect(calls.filter((call) => /^(Create|Start|Analyze|SavePostDraft)/.test(call))).toEqual([])
    },
  )
  it('keeps all choices reachable by keyboard and retains the selected channel before minting', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    const { router } = renderAppAt('/', { user: { id: 'home-alice' }, calls })
    const choices = await screen.findByRole('navigation', { name: '채널 선택' })
    const [post, naver, insta] = within(choices).getAllByRole('link')
    post!.focus()
    await user.tab()
    expect(naver).toHaveFocus()
    await user.tab()
    expect(insta).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(await screen.findByRole('textbox', { name: '클립 제목' })).toHaveValue('')
    expect(router.state.location.href).toBe('/clips/new?channel=instagram_clip')
    expect(readClipReturnContext('home-alice')).toMatchObject({
      path: '/insta-clip',
      filters: { channel: 'instagram_clip' },
    })
    expect(calls.filter((call) => /^(Create|Start|Analyze|SavePostDraft)/.test(call))).toEqual([])
  })
})
