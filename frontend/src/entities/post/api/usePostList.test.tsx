import { expect, it, vi } from 'vitest'
import { act, renderHook, waitFor } from '@testing-library/react'
import { createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import {
  contentLanguageToProto,
  GenerationJobSchema,
  ListPostsResponseSchema,
  PostService,
  PostSummarySchema,
} from '@/shared/api'
import { POLL_INTERVAL_MS } from '@/shared/config'
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

it('refreshes both history and picker while an ordinary job runs, then stops on a retained terminal snapshot', async () => {
  let failed = false
  let reads = 0
  const transport = createRouterTransport((router) => {
    router.rpc(PostService.method.listPosts, () => {
      reads += 1
      return create(ListPostsResponseSchema, {
        posts: [
          create(PostSummarySchema, {
            slug: 'running',
            targetLanguage: contentLanguageToProto('ko'),
            activeJob: create(GenerationJobSchema, {
              id: 'job',
              status: failed ? 'failed' : 'running',
              stage: 'write',
            }),
          }),
        ],
      })
    })
  })
  const queryClient = createTestQueryClient()
  const view = renderHook(() => ({ pages: usePostList({}), whole: usePosts() }), {
    wrapper: withProviders(transport, queryClient),
  })
  try {
    await waitFor(() => expect(view.result.current.pages.posts).toHaveLength(1))
    await waitFor(() => expect(view.result.current.whole.posts).toHaveLength(1))
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] })
    // Recreate polling intervals under the controlled clock after initial notifications.
    await act(() => queryClient.refetchQueries())
    failed = true
    await act(() => vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS + 100))
    await waitFor(() => expect(view.result.current.pages.posts[0].activeJob?.status).toBe('failed'))
    await waitFor(() => expect(view.result.current.whole.posts[0].activeJob?.status).toBe('failed'))
    const terminalReads = reads
    await act(() => vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS * 3))
    expect(reads).toBe(terminalReads)
  } finally {
    view.unmount()
    vi.useRealTimers()
  }
})
