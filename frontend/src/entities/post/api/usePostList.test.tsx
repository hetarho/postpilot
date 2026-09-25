import { expect, it } from 'vitest'
import { act, renderHook, waitFor } from '@testing-library/react'
import { createRouterTransport } from '@connectrpc/connect'
import { registerPostService, type FakeListRequest } from '@/test/posts'
import { createTestQueryClient, withProviders } from '@/test/session'
import { listPostsQueryKey } from './post-queries'
import { usePostList } from './usePostList'
import { usePosts } from './usePosts'

const POSTS = Array.from({ length: 25 }, (_, index) => ({
  slug: `post-${String(index).padStart(2, '0')}`,
  title: `글 ${index + 1}번`,
}))

// Every verb marks the list stale through `listPostsQueryKey` (ARCH-14). It has to reach the
// pages `/posts` scrolled through as well as the whole-list read a picker holds, or a delete would
// leave the list showing the row it removed.
it('marks the paged list and the whole-list read stale with one key', async () => {
  const listRequests: FakeListRequest[] = []
  const transport = createRouterTransport((router) =>
    registerPostService(router, { posts: POSTS, listRequests }),
  )
  const queryClient = createTestQueryClient()
  const view = renderHook(() => ({ pages: usePostList({}), whole: usePosts() }), {
    wrapper: withProviders(transport, queryClient),
  })

  await waitFor(() => expect(view.result.current.pages.posts).toHaveLength(20))
  await waitFor(() => expect(view.result.current.whole.posts).toHaveLength(25))
  act(() => view.result.current.pages.fetchNextPage())
  await waitFor(() => expect(view.result.current.pages.posts).toHaveLength(25))
  listRequests.length = 0

  await act(() => queryClient.invalidateQueries({ queryKey: listPostsQueryKey(transport) }))

  // Both loaded pages again, in order, and the unpaged read once.
  await waitFor(() => expect(listRequests).toHaveLength(3))
  expect(listRequests.filter((request) => request.pageSize === 20)).toHaveLength(2)
  expect(listRequests.filter((request) => request.pageSize === 0)).toHaveLength(1)
})

it('answers each narrowing from its own pages, newest first and without repeats', async () => {
  const transport = createRouterTransport((router) =>
    registerPostService(router, {
      posts: [...POSTS, { slug: 'jeju', title: '제주 3일', status: 'review' as const }],
    }),
  )
  const view = renderHook(({ q }) => usePostList({ q }), {
    wrapper: withProviders(transport, createTestQueryClient()),
    initialProps: { q: '' },
  })
  await waitFor(() => expect(view.result.current.posts).toHaveLength(20))
  expect(view.result.current.hasNextPage).toBe(true)

  view.rerender({ q: '제주' })
  await waitFor(() => expect(view.result.current.posts.map((post) => post.slug)).toEqual(['jeju']))
  expect(view.result.current.hasNextPage).toBe(false)
})
