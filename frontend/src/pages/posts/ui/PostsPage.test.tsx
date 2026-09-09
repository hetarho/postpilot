import { describe, expect, it } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import type { FakePostsOptions } from '@/test/posts'

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

  it('opens a durable pending AI result in the blind comparison route', async () => {
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
    await user.click(row)
    await waitFor(() =>
      expect(router.state.location.pathname).toBe('/ai-models/experiments/experiment-1'),
    )
  })

  // Plan 10 A5: a row names its voice, and a deleted one says so in words.
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

  // Plan 11 A12: an assigned row names its 템플릿 beside the voice; an unassigned one says
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

  it('offers a new draft', async () => {
    const user = userEvent.setup()
    const { router } = renderList()

    // ONE 새 글 in the tree, not one per breakpoint: the bar under the list is the same element
    // at every width, docked at every width and only narrower above the phone (THEME-24). A
    // second copy beside the heading would be a second link to the same route, so `getByRole`
    // (which throws on more than one match) is the assertion.
    const cta = await screen.findByRole('link', { name: '새 글' })
    expect(cta).toHaveAttribute('href', '/posts/new')

    await user.click(cta)

    await waitFor(() => expect(router.state.location.pathname).toBe('/posts/new'))
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

    await user.click(screen.getByRole('tab', { name: '확정' }))

    await waitFor(() => expect(router.state.location.search).toEqual({ status: 'finalized' }))
    expect(screen.getByRole('link', { name: /서울 산책/ })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /제주 3일/ })).not.toBeInTheDocument()

    await user.click(screen.getByRole('tab', { name: '전체' }))
    await waitFor(() => expect(router.state.location.search).toEqual({}))
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
    // The one action the screen exists for stays where it is (POST-64).
    expect(screen.getByRole('link', { name: '새 글' })).toBeInTheDocument()

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
    expect(screen.getByRole('tab', { name: '검토', selected: true })).toBeInTheDocument()
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
    expect(screen.getByRole('tab', { name: '전체' })).toBeInTheDocument()
  })
})
