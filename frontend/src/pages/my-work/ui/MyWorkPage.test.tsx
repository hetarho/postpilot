import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import { createFakeAuthTransport } from '@/test/session'
import type { Transport } from '@connectrpc/connect'

const clip = {
  videoTemplateId: '',
  ratio: 'vertical' as const,
  targetDurationMs: 15000,
  disclosure: 'ad' as const,
}

// Every item appears once in a counted group; the latest unfinished card is compact and emphasized.
it('counts and emphasizes the latest unfinished work inside its single group', async () => {
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
  expect(screen.queryByRole('heading', { name: '이어서 쓰기' })).toBeNull()
  const inProgress = (await screen.findByRole('heading', { name: /^작성 중/ })).closest('section')!
  const done = screen.getByRole('heading', { name: /^완성/ }).closest('section')!
  // The clip's fake timestamp is "two hours ago", newer than either post, so it leads.
  expect(within(inProgress).getByRole('link', { name: /부산 클립/ })).toHaveAttribute(
    'href',
    '/clips/clip-draft',
  )
  expect(within(inProgress).getByText('네이버 클립')).toBeInTheDocument()
  expect(within(inProgress).getByRole('link', { name: /제주 초안/ })).toHaveAttribute(
    'href',
    '/posts/draft-post',
  )
  expect(within(done).getByRole('link', { name: /서울 산책/ })).toBeInTheDocument()
  expect(screen.getAllByRole('link', { name: /부산 클립/ })).toHaveLength(1)
  // The page's one big creation leads to 만들기 (as the tab does), and reading starts nothing.
  const main = screen.getByRole('main')
  expect(within(main).getByText('최근 수정 순')).toBeInTheDocument()
  expect(within(main).getByRole('link', { name: '만들기' })).toHaveAttribute('href', '/create')
  expect(screen.getByRole('heading', { name: /^작성 중/ })).toHaveTextContent(/^작성 중\s*2$/)
  expect(
    within(inProgress)
      .getByRole('link', { name: /부산 클립/ })
      .closest('.ui-card'),
  ).toHaveClass('bg-badge-accent-bg')
  expect(main.querySelectorAll('.ui-card.bg-badge-accent-bg')).toHaveLength(1)
  // Creation remains the one filled CTA.
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

it('waits for both histories without exposing a partial latest item or replaying the body', async () => {
  let release!: () => void
  const waiting = new Promise<void>((resolve) => {
    release = resolve
  })
  const base = createFakeAuthTransport({
    existingSetup: true,
    user: { id: 'alice' },
    posts: { posts: [{ slug: 'waiting-post', title: '먼저 도착한 글', status: 'draft' }] },
    clips: { projects: [{ ...clip, id: 'waiting-clip', title: '나중에 도착한 영상' }] },
  })
  const transport: Transport = new Proxy(base, {
    get(target, property) {
      if (property !== 'unary') return Reflect.get(target, property)
      return async (...args: unknown[]) => {
        if ((args[0] as { name: string }).name === 'ListClipProjects') await waiting
        return Reflect.apply(target.unary, target, args)
      }
    },
  })
  const view = renderAppAt('/my', { transport })
  const main = await screen.findByRole('main')
  await waitFor(() => expect(main.querySelector('[data-work-skeleton]')).not.toBeNull())
  const body = main.querySelector('[data-work-body]')
  expect(within(main).queryByRole('link', { name: /먼저 도착한 글/ })).toBeNull()
  expect(within(main).queryByRole('heading', { name: '이어서 쓰기' })).toBeNull()
  await act(async () => {
    release()
    await waiting
  })
  await screen.findByRole('link', { name: /나중에 도착한 영상/ })
  expect(within(main).getByRole('link', { name: /먼저 도착한 글/ })).toBeInTheDocument()
  expect(main.querySelector('[data-work-body]')).toBe(body)
  expect(main.querySelector('[data-work-skeleton]')).toBeNull()
  expect(view.router.state.location.pathname).toBe('/my')
})

it('keeps both channels’ prepared stories unfinished and gives cancelled attempts precedence', async () => {
  const storyline = {
    paragraphs: [{ text: '직접 겪은 이야기', observationIds: [] }],
    editedByHand: false,
    addedSourceIds: [],
    takenOutObservationIds: [],
  }
  renderAppAt('/my', {
    user: { id: 'alice' },
    clips: {
      projects: [
        { ...clip, id: 'naver-story', title: '네이버 이야기', channel: 'naver_clip', storyline },
        {
          ...clip,
          id: 'insta-story',
          title: '인스타 이야기',
          channel: 'instagram_clip',
          storyline,
          dubbing: { enabled: true, voiceId: 'confirmed' },
        },
        {
          ...clip,
          id: 'cancelled-story',
          title: '취소된 이야기',
          storyline,
          latestJob: {
            id: 'cancelled',
            kind: 'storyline_clip',
            status: 'cancelled',
            stage: 'storyline',
          },
        },
      ],
    },
  })
  expect(await screen.findByRole('link', { name: /네이버 이야기/ })).toHaveTextContent(
    '스토리라인 작성 완료',
  )
  expect(await screen.findByRole('link', { name: /인스타 이야기/ })).toHaveTextContent(
    '스토리라인 작성 완료',
  )
  expect(await screen.findByRole('link', { name: /취소된 이야기/ })).toHaveTextContent('취소됨')
  expect(screen.queryByRole('heading', { name: /^완성/ })).toBeNull()
})

it('keeps completed-only work in 완성 without a continuation placeholder', async () => {
  renderAppAt('/my', {
    user: { id: 'alice' },
    posts: { posts: [{ slug: 'done-only', title: '완성한 글', status: 'finalized' }] },
    clips: { projects: [] },
  })
  const group = (await screen.findByRole('heading', { name: /^완성/ })).closest('section')!
  expect(within(group).getByRole('link', { name: /완성한 글/ })).toBeInTheDocument()
  expect(screen.queryByRole('heading', { name: /^작성 중|이어서 쓰기/ })).toBeNull()
  expect(screen.getByRole('main').querySelectorAll('.ui-card.bg-badge-accent-bg')).toHaveLength(0)
})
it('points an empty aggregate to its actual creation destination', async () => {
  renderAppAt('/my', { user: { id: 'alice' }, posts: { posts: [] }, clips: { projects: [] } })
  expect(
    await screen.findByText('아직 만든 글이나 영상이 없어요. 만들기에서 시작해 보세요.'),
  ).toBeInTheDocument()
  expect(screen.queryByRole('heading', { name: /^작성 중|이어서 쓰기/ })).toBeNull()
  expect(within(screen.getByRole('main')).getByRole('link', { name: '만들기' })).toHaveAttribute(
    'href',
    '/create',
  )
})
it('retains a failed aggregate read and its retry without inventing resumable work', async () => {
  renderAppAt('/my', {
    user: { id: 'alice' },
    posts: { posts: [] },
    clips: { projectListFails: true },
  })
  expect(await screen.findByRole('alert')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '다시 시도' })).toBeInTheDocument()
  expect(screen.queryByRole('heading', { name: /^작성 중|이어서 쓰기/ })).toBeNull()
})
