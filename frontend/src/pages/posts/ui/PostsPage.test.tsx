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

  // Plan 11 A12: an assigned row names its 용도 beside the voice; an unassigned one says
  // nothing at all, since 없음 is the majority of the list.
  it("names an assigned row's purpose and leaves an unassigned row alone", async () => {
    renderList({
      posts: [
        {
          slug: '20260828-jeju',
          title: '제주 3일',
          purpose: { id: 'purpose-review', name: '정보성 식당 리뷰' },
        },
        { slug: '20260820-plain', title: '용도 없는 글' },
      ],
    })

    expect(await screen.findByRole('link', { name: /제주 3일/ })).toHaveTextContent(
      '정보성 식당 리뷰',
    )
    const plain = screen.getByRole('link', { name: /용도 없는 글/ })
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
    // at every width — a dock in the thumb's band on a phone, the list's last row above it. A
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
})
