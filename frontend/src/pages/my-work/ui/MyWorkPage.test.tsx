import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'

const clip = {
  videoTemplateId: '',
  ratio: 'vertical' as const,
  targetDurationMs: 15000,
  disclosure: 'ad' as const,
}

// Guide: 내 글 holds every post and every clip. The newest unfinished work leads as 이어서 쓰기;
// the rest is grouped 작성 중 / 완성, each item once, with its channel named.
it('leads with the newest unfinished work and groups posts and clips by status', async () => {
  const calls: string[] = []
  renderAppAt('/my', {
    user: { id: 'alice' },
    calls,
    posts: {
      posts: [
        {
          slug: 'draft-post',
          title: '제주 초안',
          status: 'draft',
          updatedAt: '2026-11-09T00:00:00Z',
        },
        {
          slug: 'done-post',
          title: '서울 산책',
          status: 'finalized',
          updatedAt: '2026-11-01T00:00:00Z',
        },
      ],
    },
    clips: { projects: [{ ...clip, id: 'clip-draft', title: '부산 클립' }] },
  })

  expect(await screen.findByRole('heading', { level: 1, name: '내 글' })).toBeInTheDocument()
  const hero = (await screen.findByRole('heading', { name: '이어서 쓰기' })).closest('section')!
  const inProgress = (await screen.findByRole('heading', { name: /^작성 중/ })).closest('section')!
  const done = screen.getByRole('heading', { name: /^완성/ }).closest('section')!
  // The clip's fake timestamp is "two hours ago", newer than either post, so it leads.
  expect(within(hero).getByRole('link', { name: /부산 클립/ })).toHaveAttribute(
    'href',
    '/clips/clip-draft',
  )
  expect(within(hero).getByText('네이버 클립')).toBeInTheDocument()
  expect(within(inProgress).getByRole('link', { name: /제주 초안/ })).toHaveAttribute(
    'href',
    '/posts/draft-post',
  )
  expect(within(done).getByRole('link', { name: /서울 산책/ })).toBeInTheDocument()
  expect(screen.getAllByRole('link', { name: /부산 클립/ })).toHaveLength(1)
  // The page's one big creation leads to 만들기 (as the tab does), and reading starts nothing.
  const main = screen.getByRole('main')
  expect(within(main).getByRole('link', { name: '만들기' })).toHaveAttribute('href', '/create')
  // THEME-18: the screen's one filled CTA is 만들기; the tinted hero's 계속하기 is not a second.
  expect([...main.querySelectorAll('.ui-button-cta')].map((node) => node.textContent)).toEqual([
    '만들기',
  ])
  expect(calls.filter((call) => /^(Start|Create|Generate|Save)/.test(call))).toEqual([])
})

// THEME-58 →THEME-69: work opened from 내 글 전체 returns to it — its way back names 내 글 and
// lands where the owner left the list — not to the channel's own list.
describe('returning to 내 글', () => {
  let scrollY: PropertyDescriptor | undefined
  beforeEach(() => {
    scrollY = Object.getOwnPropertyDescriptor(window, 'scrollY')
    Object.defineProperty(window, 'scrollY', { configurable: true, value: 640 })
  })
  afterEach(() => {
    if (scrollY) Object.defineProperty(window, 'scrollY', scrollY)
    sessionStorage.clear()
    vi.restoreAllMocks()
  })
  const fixtures = {
    user: { id: 'alice' },
    posts: {
      posts: [
        {
          slug: 'draft-post',
          title: '제주 초안',
          status: 'draft',
          updatedAt: '2026-11-09T00:00:00Z',
        },
      ],
    },
    clips: { projects: [{ ...clip, id: 'clip-draft', title: '부산 클립' }] },
  }

  it.each([
    ['a post', /제주 초안/, '/posts/draft-post'],
    ['a clip', /부산 클립/, '/clips/clip-draft'],
  ])('brings %s back to 내 글 at the scroll it left', async (_, name, href) => {
    const user = userEvent.setup()
    const scroll = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
    const { router } = renderAppAt('/my', fixtures)
    await user.click(await screen.findByRole('link', { name }))
    await waitFor(() => expect(router.state.location.pathname).toBe(href))
    const back = await screen.findByRole('link', { name: '내 글로 돌아가기' })
    expect(back).toHaveAttribute('href', '/my')
    scroll.mockClear()
    await user.click(back)
    await waitFor(() => expect(router.state.location.pathname).toBe('/my'))
    await waitFor(() => expect(scroll).toHaveBeenCalledWith(0, 640))
  })
})
