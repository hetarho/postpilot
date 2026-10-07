import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import type { FakeListRequest, FakePostRow, FakePostsOptions } from '@/test/posts'
import { POST_CONTENT_FIXTURE } from '@/test/fixtures/postContent'
import {
  markPostHistoryReturn,
  readPostHistoryReturn,
  readPostReturnContext,
  rememberPostEntry,
} from '@/entities/post'

const USER = { id: 'alice' }

/** Renders the list screen through the real route tree, so the row links are the ones the
 *  router actually resolves. */
function renderList(posts: FakePostsOptions = {}) {
  return renderAppAt('/posts', { user: USER, posts })
}

describe('PostsPage', () => {
  // A4.
  it('lists the posts the server returned, in that order', async () => {
    renderList({
      posts: [
        { slug: '20260828-jeju', title: '제주 3일', updatedAt: '2026-08-28T11:58:00Z' },
        { slug: '20260820-busan', title: '부산', status: 'review' },
      ],
    })

    const rows = await screen.findAllByRole('link', { name: /제주 3일|부산/ })
    expect(rows).toHaveLength(2)
    expect(rows[0]).toHaveTextContent('제주 3일')
    expect(rows[1]).toHaveTextContent('부산')
  })

  it('shows a status badge per row', async () => {
    renderList({
      posts: [
        { slug: '20260828-jeju', title: '제주 3일' },
        { slug: '20260820-busan', title: '부산', status: 'review' },
      ],
    })

    expect(await screen.findByRole('link', { name: /제주 3일/ })).toHaveTextContent('초안')
    expect(screen.getByRole('link', { name: /부산/ })).toHaveTextContent('검토')
  })

  it('marks a post with an active job as generating', async () => {
    renderList({
      posts: [
        {
          slug: '20260828-jeju',
          title: '제주 3일',
          activeJob: { id: 'job-1', status: 'running', stage: 'observe' },
        },
      ],
    })

    expect(await screen.findByRole('link', { name: /제주 3일/ })).toHaveTextContent('AI 생성 중')
  })

  it('opens a durable pending AI result in the canonical retained-record route', async () => {
    const user = userEvent.setup()
    const { router } = renderList({
      posts: [
        {
          slug: '20260828-jeju',
          title: '제주 3일',
          pendingExperimentId: 'experiment-1',
        },
      ],
    })

    const row = await screen.findByRole('link', { name: /제주 3일/ })
    expect(row).toHaveTextContent('AI 결과 확인')
    expect(row).toHaveAttribute('href', '/tests/records/experiment-1?entry=%2Fposts')
    await user.click(row)
    await waitFor(() => expect(router.state.location.pathname).toBe('/tests/records/experiment-1'))
    expect(router.state.location.search.entry).toBe('/posts')
  })

  // POST-25: a row names its voice, and a deleted one says so in words.
  it("names each row's voice and marks a deleted one as a tombstone", async () => {
    renderList({
      posts: [
        { slug: '20260828-jeju', title: '제주 3일', voice: { id: 'voice-review', name: '리뷰' } },
        {
          slug: '20260820-old',
          title: '옛 글',
          voice: { id: 'voice-old', name: '옛 말투', deleted: true },
        },
      ],
    })

    expect(await screen.findByRole('link', { name: /제주 3일/ })).toHaveTextContent('리뷰')
    expect(screen.getByRole('link', { name: /옛 글/ })).toHaveTextContent('삭제된 말투 · 옛 말투')
  })

  // POST-25: 말투 없음 is an absence, not a value, so the row says nothing about a voice — neither
  // a name nor the voice's language badge, and no '말투 없음' either.
  it('shows no voice on the row of a post with 말투 없음', async () => {
    renderList({
      posts: [
        { slug: '20260828-jeju', title: '제주 3일' },
        { slug: '20260820-plain', title: '말투 없는 글', voice: null },
      ],
    })

    const voiced = await screen.findByRole('link', { name: /제주 3일/ })
    expect(voiced).toHaveTextContent('기본 말투')
    // A voice carries no language, so the row shows no language chip (VOICE-10).
    expect(voiced).not.toHaveTextContent('한국어')
    const plain = screen.getByRole('link', { name: /말투 없는 글/ })
    expect(plain).toHaveTextContent('초안')
    expect(plain).not.toHaveTextContent('기본 말투')
    expect(plain).not.toHaveTextContent('말투 없음')
    expect(plain).not.toHaveTextContent('한국어')
  })

  // TMPL-32: an assigned row names its 템플릿 beside the voice; an unassigned one says
  // nothing at all, since 없음 is the majority of the list.
  it("names an assigned row's template and leaves an unassigned row alone", async () => {
    renderList({
      posts: [
        {
          slug: '20260828-jeju',
          title: '제주 3일',
          template: { id: 'template-review', name: '정보성 식당 리뷰' },
        },
        { slug: '20260820-plain', title: '템플릿 없는 글' },
      ],
    })

    expect(await screen.findByRole('link', { name: /제주 3일/ })).toHaveTextContent(
      '정보성 식당 리뷰',
    )
    const plain = screen.getByRole('link', { name: /템플릿 없는 글/ })
    expect(plain).toHaveTextContent('기본 말투')
    expect(plain).not.toHaveTextContent('정보성 식당 리뷰')
  })

  it('labels a post nobody has titled yet', async () => {
    renderList({ posts: [{ slug: '20260828-untitled', title: '' }] })

    expect(await screen.findByRole('link', { name: /제목 없음/ })).toBeInTheDocument()
  })

  it('opens the editor for the row that was clicked', async () => {
    const user = userEvent.setup()
    const { router } = renderList({ posts: [{ slug: '20260828-jeju', title: '제주 3일' }] })

    await user.click(await screen.findByRole('link', { name: /제주 3일/ }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/posts/20260828-jeju'))
  })

  it('keeps history secondary and offers no docked new-writing action', async () => {
    renderList()
    await screen.findByText(/아직 글이 없어요/)
    expect(screen.queryByRole('link', { name: '새 글' })).not.toBeInTheDocument()
    expect(screen.queryByRole('region', { name: '글 작성' })).not.toBeInTheDocument()
  })

  it('uses canonical content for separate continuation and export actions without reading each post', async () => {
    const calls: string[] = []
    renderList({
      calls,
      posts: [
        {
          slug: 'review-content',
          title: '내용 있는 검토 글',
          status: 'review',
          content: POST_CONTENT_FIXTURE,
        },
        { slug: 'empty-finalized', title: '내용 없는 확정 글', status: 'finalized' },
        {
          slug: 'published-content',
          title: '발행한 작업',
          status: 'published',
          content: POST_CONTENT_FIXTURE,
          publishedUrl: 'https://blog.naver.com/alice/1',
        },
      ],
    })
    const history = await screen.findByRole('list', { name: '글 작업 내역' })
    const [review, empty, published] = await within(history).findAllByRole('listitem')
    const resume = within(review).getByRole('link', { name: '이어서 작성' })
    const exportAction = within(review).getByRole('link', { name: '내보내기' })
    expect(resume).toHaveAttribute('href', '/posts/review-content')
    expect(exportAction).toHaveAttribute('href', '/posts/review-content')
    expect(exportAction.closest('a')?.parentElement?.closest('a')).toBeNull()
    expect(within(empty).queryByRole('link', { name: '내보내기' })).not.toBeInTheDocument()
    expect(within(published).getByRole('link', { name: '발행한 글' })).toHaveAttribute(
      'href',
      'https://blog.naver.com/alice/1',
    )
    expect(calls).not.toContain('GetPost')
    expect(within(history).queryByText(POST_CONTENT_FIXTURE.summary)).not.toBeInTheDocument()
  })

  it('prioritizes an ordinary failure without hiding a retained test result or usable content', async () => {
    renderList({
      posts: [
        {
          slug: 'failed',
          title: '다시 쓸 작업',
          status: 'review',
          content: POST_CONTENT_FIXTURE,
          pendingExperimentId: 'legacy-result',
          latestOrdinaryFailure: {
            id: 'failed-write',
            kind: 'generate_post',
            status: 'failed',
            stage: 'write',
            failureReason: 'NETWORK_UNAVAILABLE',
          },
        },
      ],
    })
    const history = await screen.findByRole('list', { name: '글 작업 내역' })
    const work = await within(history).findByRole('link', { name: /다시 쓸 작업/ })
    expect(work).toHaveTextContent('AI 결과 오류')
    expect(work).toHaveAttribute('href', '/posts/failed')
    expect(within(history).getByRole('link', { name: '내보내기' })).toBeInTheDocument()
    expect(within(history).getByRole('link', { name: '이전 AI 결과 확인' })).toHaveAttribute(
      'href',
      '/tests/records/legacy-result?entry=%2Fposts',
    )
  })

  it('retains the history narrowing and export intent when opening a content-bearing draft', async () => {
    const user = userEvent.setup()
    const { router } = renderAppAt('/posts?q=제주&status=review', {
      user: USER,
      posts: {
        posts: [
          { slug: 'jeju', title: '제주 작업', status: 'review', content: POST_CONTENT_FIXTURE },
        ],
      },
    })
    await user.click(await screen.findByRole('link', { name: '내보내기' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/posts/jeju'))
    expect(readPostReturnContext(USER.id, 'jeju')).toMatchObject({
      path: '/posts',
      targetId: 'jeju',
      filters: { q: '제주', status: 'review', intent: 'export' },
    })
  })

  it('says so when there is nothing yet', async () => {
    renderList()

    expect(await screen.findByText(/아직 글이 없어요/)).toBeInTheDocument()
  })

  it('says so when the list cannot be loaded', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderList({ listFails: true, calls })

    expect(await screen.findByRole('alert')).toHaveTextContent('목록을 불러오지 못했어요')
    await user.click(screen.getByRole('button', { name: '다시 시도' }))
    await waitFor(() =>
      expect(calls.filter((call) => call === 'ListPosts').length).toBeGreaterThan(1),
    )
  })

  const NARROWABLE: FakePostsOptions = {
    posts: [
      { slug: '20260828-jeju', title: '제주 3일', status: 'review', tags: ['제주', '카페'] },
      { slug: '20260820-busan', title: '부산 밥상', status: 'draft', tags: ['맛집'] },
      { slug: '20260810-seoul', title: '서울 산책', status: 'finalized' },
    ],
  }

  // POST-66: on `post.status`, and 전체 takes the param back out of the URL.
  it('narrows by status and carries the choice in the URL', async () => {
    const user = userEvent.setup()
    const { router } = renderList(NARROWABLE)
    await screen.findByRole('link', { name: /제주 3일/ })

    await user.click(screen.getByRole('combobox', { name: /^상태/ }))
    await user.click(await screen.findByRole('option', { name: '확정' }))

    await waitFor(() => expect(router.state.location.search).toEqual({ status: 'finalized' }))
    expect(screen.getByRole('link', { name: /서울 산책/ })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /제주 3일/ })).not.toBeInTheDocument()

    await user.click(screen.getByRole('combobox', { name: /^상태/ }))
    await user.click(await screen.findByRole('option', { name: '전체' }))
    await waitFor(() => expect(router.state.location.search).toEqual({}))
  })

  const WITH_PUBLISHED: FakePostsOptions = {
    posts: [
      ...(NARROWABLE.posts ?? []),
      {
        slug: '20260801-seongsu',
        title: '성수 카페',
        status: 'published',
        publishedUrl: 'https://blog.naver.com/alice/1',
        publishedAt: '2026-08-02T09:00:00Z',
      },
    ],
  }

  // Published is finished for good: its label tells it from 확정 (THEME-29), and its chip takes the
  // solid plane where 확정 keeps the tint, so the two do not read alike at a glance.
  it('wears 발행됨 on a published row, on a stronger chip than 확정', async () => {
    renderList(WITH_PUBLISHED)

    expect(await screen.findByRole('link', { name: /성수 카페/ })).toHaveTextContent('발행됨')
    expect(screen.getByRole('link', { name: /서울 산책/ })).toHaveTextContent('확정')
    expect(screen.getByRole('link', { name: /서울 산책/ })).not.toHaveTextContent('발행됨')
    expect(screen.getByText('발행됨', { selector: 'span' })).toHaveClass('bg-badge-done-bg')
    expect(screen.getByText('확정', { selector: 'span' })).toHaveClass('bg-notice-success-bg')
  })

  // POST-66: the filter offers every status in lifecycle order, and 발행됨 narrows to it alone.
  it('narrows to the published posts, and the address survives a reload', async () => {
    const user = userEvent.setup()
    const { router, unmount } = renderList(WITH_PUBLISHED)
    await screen.findByRole('link', { name: /성수 카페/ })

    await user.click(screen.getByRole('combobox', { name: /^상태/ }))
    expect((await screen.findAllByRole('option')).map((option) => option.textContent)).toEqual([
      '전체',
      '초안',
      '검토',
      '확정',
      '발행됨',
    ])
    await user.click(screen.getByRole('option', { name: '발행됨' }))

    await waitFor(() => expect(router.state.location.search).toEqual({ status: 'published' }))
    expect(screen.getByRole('link', { name: /성수 카페/ })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /서울 산책/ })).toBeNull()
    const address = router.state.location.href
    unmount()

    renderAppAt(address, { user: USER, posts: WITH_PUBLISHED })
    expect(await screen.findByRole('link', { name: /성수 카페/ })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /제주 3일/ })).toBeNull()
    expect(screen.getByRole('combobox', { name: /^상태/ })).toHaveTextContent('발행됨')
  })

  it('honours ?status=published on load', async () => {
    renderAppAt('/posts?status=published', { user: USER, posts: WITH_PUBLISHED })

    expect(await screen.findByRole('link', { name: /성수 카페/ })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /서울 산책/ })).toBeNull()
    expect(screen.queryByRole('link', { name: /부산 밥상/ })).toBeNull()
    expect(screen.getByRole('combobox', { name: /^상태/ })).toHaveTextContent('발행됨')
  })

  // POST-65: the tag is why the row is still here, so the row says which tag it was.
  it('searches the tags and names the tag that matched', async () => {
    const user = userEvent.setup()
    renderList(NARROWABLE)
    await screen.findByRole('link', { name: /제주 3일/ })

    await user.type(screen.getByLabelText('검색'), '카페')

    await waitFor(() => expect(screen.queryByRole('link', { name: /부산 밥상/ })).toBeNull())
    const row = screen.getByRole('link', { name: /제주 3일/ })
    expect(row).toHaveTextContent('#카페')
    // Only the tag that matched — the row is not a tag list.
    expect(row).not.toHaveTextContent('#제주')
  })

  it('leaves the row alone when the title is what matched', async () => {
    const user = userEvent.setup()
    renderList(NARROWABLE)
    await screen.findByRole('link', { name: /제주 3일/ })

    await user.type(screen.getByLabelText('검색'), '밥상')

    const row = await screen.findByRole('link', { name: /부산 밥상/ })
    expect(row.textContent).not.toContain('#')
  })

  // POST-69: not the same screen as an account with no posts, and it names the narrowing.
  it('names the narrowing that matched nothing and clears it on request', async () => {
    const user = userEvent.setup()
    const { router } = renderList(NARROWABLE)
    await screen.findByRole('link', { name: /제주 3일/ })

    await user.type(screen.getByLabelText('검색'), '없는말')

    expect(await screen.findByText(/"없는말"에 맞는 글이 없어요/)).toBeInTheDocument()
    expect(screen.queryByText(/아직 글이 없어요/)).toBeNull()
    expect(screen.queryByRole('link', { name: '새 글' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '초기화' }))

    await waitFor(() => expect(router.state.location.search).toEqual({}))
    expect(await screen.findByRole('link', { name: /제주 3일/ })).toBeInTheDocument()
  })

  // POST-67: the URL is the source, so a reload or a shared link starts narrowed.
  it('starts narrowed from the URL', async () => {
    renderAppAt('/posts?q=제주&status=review', { user: USER, posts: NARROWABLE })

    expect(await screen.findByRole('link', { name: /제주 3일/ })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /부산 밥상/ })).toBeNull()
    expect(screen.getByLabelText('검색')).toHaveValue('제주')
    expect(screen.getByRole('combobox', { name: /^상태/ })).toHaveTextContent('검토')
  })

  // POST-67: typing replaces the entry it is on. Otherwise 뒤로 walks back one character at a
  // time instead of leaving the list.
  it('does not push a history entry per keystroke', async () => {
    const user = userEvent.setup()
    const { router } = renderList(NARROWABLE)
    await screen.findByRole('link', { name: /제주 3일/ })
    const before = router.history.length

    await user.type(screen.getByLabelText('검색'), '제주')

    await waitFor(() => expect(router.state.location.search).toEqual({ q: '제주' }))
    expect(router.history.length).toBe(before)
  })

  // POST-68: no threshold — the controls are the same page at three posts and at none.
  it('keeps the search and the filter on an empty account', async () => {
    renderList()

    expect(await screen.findByText(/아직 글이 없어요/)).toBeInTheDocument()
    expect(screen.getByLabelText('검색')).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: /^상태/ })).toHaveTextContent('전체')
  })
})

describe('PostsPage paging (POST-90..92)', () => {
  // jsdom has no IntersectionObserver. This one keeps every live observer so a test can say the
  // list's end came within a screen, which is what the page listens to.
  const live = new Set<(entries: Array<Partial<IntersectionObserverEntry>>) => void>()
  beforeEach(() => {
    live.clear()
    vi.stubGlobal(
      'IntersectionObserver',
      class {
        private readonly callback: (entries: Array<Partial<IntersectionObserverEntry>>) => void
        constructor(callback: (entries: Array<Partial<IntersectionObserverEntry>>) => void) {
          this.callback = callback
        }
        observe() {
          live.add(this.callback)
        }
        disconnect() {
          live.delete(this.callback)
        }
      },
    )
  })
  afterEach(() => vi.unstubAllGlobals())

  function reportNearEnd() {
    act(() => {
      for (const callback of [...live]) callback([{ isIntersecting: true }])
    })
  }

  /** `count` posts, newest first, the last one titled `last` when given. */
  function manyPosts(count: number, last?: Partial<FakePostRow>): FakePostRow[] {
    return Array.from({ length: count }, (_, index) => ({
      slug: `post-${String(index).padStart(2, '0')}`,
      title: `글 ${index + 1}번`,
      ...(index === count - 1 ? last : {}),
    }))
  }

  const rowCount = () => screen.getAllByRole('link', { name: /글 \d+번|제주/ }).length

  it('restores an explicit cold-cache return after the page containing its target arrives', async () => {
    rememberPostEntry(USER.id, {
      path: '/posts',
      section: 'posts',
      filters: {},
      scrollY: 1400,
      targetId: 'post-24',
    })
    expect(markPostHistoryReturn(USER.id, 'post-24')).toBe(true)
    let release!: () => void
    const listPageGate = new Promise<void>((resolve) => (release = resolve))
    const listRequests: FakeListRequest[] = []
    const scroll = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
    const view = renderList({ posts: manyPosts(25), listPageGate, listRequests })
    try {
      await screen.findByRole('link', { name: /글 20번/ })
      expect(await screen.findByText('불러오는 중…')).toBeInTheDocument()
      expect(scroll).not.toHaveBeenCalledWith(0, 1400)
      expect(readPostHistoryReturn(USER.id)).toBeDefined()
      release()
      const last = await screen.findByRole('link', { name: /글 25번/ })
      expect(last.closest('li')).toHaveAttribute('data-post-slug', 'post-24')
      await waitFor(() => expect(scroll).toHaveBeenCalledWith(0, 1400))
      expect(readPostHistoryReturn(USER.id)).toBeUndefined()
      expect(listRequests).toHaveLength(2)
    } finally {
      release()
      view.unmount()
      scroll.mockRestore()
    }
  })

  it('waits for sufficient rendered history height even when the edited target is on the first page', async () => {
    rememberPostEntry(USER.id, {
      path: '/posts',
      section: 'posts',
      filters: {},
      scrollY: 1400,
      targetId: 'post-00',
    })
    expect(markPostHistoryReturn(USER.id, 'post-00')).toBe(true)
    let release!: () => void
    const listPageGate = new Promise<void>((resolve) => (release = resolve))
    const listRequests: FakeListRequest[] = []
    const scroll = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
    const originalHeight = Object.getOwnPropertyDescriptor(document.documentElement, 'scrollHeight')
    vi.stubGlobal('innerHeight', 900)
    // jsdom has no layout; derive the available height from committed history rows.
    // Page 1 is too short, while page 2 is sufficient with page 3 still unloaded.
    Object.defineProperty(document.documentElement, 'scrollHeight', {
      configurable: true,
      get: () => (document.querySelectorAll('[data-post-slug]').length >= 40 ? 2600 : 1100),
    })
    const view = renderList({ posts: manyPosts(45), listPageGate, listRequests })
    try {
      expect((await screen.findByRole('link', { name: /글 1번/ })).closest('li')).toHaveAttribute(
        'data-post-slug',
        'post-00',
      )
      expect(await screen.findByText('불러오는 중…')).toBeInTheDocument()
      expect(rowCount()).toBe(20)
      expect(scroll).not.toHaveBeenCalledWith(0, 1400)
      expect(readPostHistoryReturn(USER.id)).toBeDefined()
      release()
      await screen.findByRole('link', { name: /글 40번/ })
      await waitFor(() => expect(scroll).toHaveBeenCalledWith(0, 1400))
      expect(readPostHistoryReturn(USER.id)).toBeUndefined()
      expect(rowCount()).toBe(40)
      expect(screen.queryByRole('link', { name: /글 45번/ })).not.toBeInTheDocument()
      expect(listRequests).toHaveLength(2)
    } finally {
      release()
      view.unmount()
      scroll.mockRestore()
      if (originalHeight)
        Object.defineProperty(document.documentElement, 'scrollHeight', originalHeight)
      else Reflect.deleteProperty(document.documentElement, 'scrollHeight')
    }
  })

  it('loads the first page, the next as the end nears, and nothing past the last', async () => {
    const listRequests: FakeListRequest[] = []
    renderList({ posts: manyPosts(25), listRequests })

    await screen.findByRole('link', { name: /글 20번/ })
    expect(rowCount()).toBe(20)
    expect(listRequests).toEqual([{ pageSize: 20, pageToken: '', query: '', status: '' }])

    reportNearEnd()
    expect(await screen.findByRole('link', { name: /글 25번/ })).toBeInTheDocument()
    expect(rowCount()).toBe(25)
    expect(listRequests).toHaveLength(2)

    // The last page came back with no token: the end is no longer watched, so nothing more is asked.
    reportNearEnd()
    await waitFor(() => expect(live.size).toBe(0))
    expect(listRequests).toHaveLength(2)
  })

  it('says a page is loading at the list end while it is', async () => {
    let release!: () => void
    const listPageGate = new Promise<void>((resolve) => (release = resolve))
    renderList({ posts: manyPosts(25), listPageGate })
    await screen.findByRole('link', { name: /글 20번/ })

    reportNearEnd()
    expect(await screen.findByText('불러오는 중…')).toBeInTheDocument()

    release()
    expect(await screen.findByRole('link', { name: /글 25번/ })).toBeInTheDocument()
    expect(screen.queryByText('불러오는 중…')).toBeNull()
  })

  // POST-92: the rows on screen are still right, so a failure further down keeps them.
  it('keeps every row when the next page fails and retries only that page', async () => {
    const user = userEvent.setup()
    const listRequests: FakeListRequest[] = []
    renderList({ posts: manyPosts(25), listPageFailures: 1, listRequests })
    await screen.findByRole('link', { name: /글 20번/ })

    reportNearEnd()
    expect(await screen.findByText('더 불러오지 못했어요.')).toBeInTheDocument()
    expect(rowCount()).toBe(20)
    expect(screen.queryByText('목록을 불러오지 못했어요.')).toBeNull()
    // A failed page waits for the button instead of retrying on every scroll.
    expect(live.size).toBe(0)

    await user.click(screen.getByRole('button', { name: '다시 시도' }))
    expect(await screen.findByRole('link', { name: /글 25번/ })).toBeInTheDocument()
    expect(screen.queryByText('더 불러오지 못했어요.')).toBeNull()
    expect(listRequests.map((request) => request.pageToken === '')).toEqual([true, false, false])
  })

  // POST-91: the search reaches posts the list has not loaded.
  it('finds a post that sits past the loaded rows', async () => {
    const user = userEvent.setup()
    renderList({ posts: manyPosts(25, { title: '제주 마지막', tags: ['여행'] }) })
    await screen.findByRole('link', { name: /글 20번/ })
    expect(screen.queryByRole('link', { name: /제주 마지막/ })).toBeNull()

    await user.type(screen.getByLabelText('검색'), '제주')

    expect(await screen.findByRole('link', { name: /제주 마지막/ })).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByRole('link', { name: /글 \d+번/ })).toBeNull())
  })

  it('sends a typed word once it settles, not a request per keystroke', async () => {
    const user = userEvent.setup()
    const listRequests: FakeListRequest[] = []
    const { router } = renderList({ posts: manyPosts(3), listRequests })
    await screen.findByRole('link', { name: /글 3번/ })

    await user.type(screen.getByLabelText('검색'), '글 2번')

    // The URL follows every keystroke (POST-67) …
    expect(router.state.location.search).toEqual({ q: '글 2번' })
    // … and the request waits for the typing to stop.
    await screen.findByRole('link', { name: /글 2번/ })
    await waitFor(() => expect(screen.queryByRole('link', { name: /글 3번/ })).toBeNull())
    expect(listRequests.map((request) => request.query)).toEqual(['', '글 2번'])
  })

  // POST-69: a narrowing that matches nothing says so even on an empty account, and clearing it
  // then says the account has no posts.
  it('names the narrowing on an empty account and the emptiness once it is cleared', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts?q=제주', { user: USER, posts: {} })

    expect(await screen.findByText(/"제주"에 맞는 글이 없어요/)).toBeInTheDocument()
    expect(screen.queryByText(/아직 글이 없어요/)).toBeNull()

    await user.click(screen.getByRole('button', { name: '초기화' }))
    expect(await screen.findByText(/아직 글이 없어요/)).toBeInTheDocument()
  })
})
