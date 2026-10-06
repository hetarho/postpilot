import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { cleanup, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { renderAppAt } from '@/test/app'
import { createThemeTestEnvironment } from '@/test/theme'

beforeEach(() => initializeI18n('ko'))
afterEach(() => {
  cleanup()
  localStorage.clear()
  initializeI18n('ko')
})

describe('creation launch', () => {
  it.each(['light', 'dark'] as const)(
    'offers exactly two named destinations in the %s theme without creating work',
    async (storedPreference) => {
      const calls: string[] = []
      const theme = createThemeTestEnvironment({ storedPreference })
      renderAppAt('/', { user: { id: 'home-alice' }, calls, theme: theme.ports })
      const choices = await screen.findByRole('navigation', { name: '새로 만들기' })
      const links = within(choices).getAllByRole('link')
      expect(links).toHaveLength(2)
      expect(links[0]).toHaveAccessibleName('새 글 작성하기')
      expect(links[0]).toHaveAccessibleDescription('사진과 메모를 나다운 글로')
      expect(links[0]).toHaveAttribute('href', '/posts/new')
      expect(links[1]).toHaveAccessibleName('새 클립 만들기')
      expect(links[1]).toHaveAccessibleDescription('일상의 장면을 하나의 영상으로')
      expect(links[1]).toHaveAttribute('href', '/clips/new')
      expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1)
      expect(screen.queryByRole('combobox')).toBeNull()
      expect(calls.filter((call) => /^(Create|Start|Analyze|SavePostDraft)/.test(call))).toEqual([])
    },
  )

  it('keeps both choices reachable in reading order and opens an empty editor through the keyboard', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    const { router } = renderAppAt('/', { user: { id: 'home-alice' }, calls })
    const choices = await screen.findByRole('navigation', { name: '새로 만들기' })
    const [post, clip] = within(choices).getAllByRole('link')
    post!.focus()
    await user.tab()
    expect(clip).toHaveFocus()
    await user.tab({ shift: true })
    expect(post).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(await screen.findByLabelText('제목')).toHaveValue('')
    expect(router.state.location.pathname).toBe('/posts/new')
    expect(calls.filter((call) => /^(Create|Start|Analyze|SavePostDraft)/.test(call))).toEqual([])
  })
})
