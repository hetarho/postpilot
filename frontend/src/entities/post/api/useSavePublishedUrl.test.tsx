import { create } from '@bufbuild/protobuf'
import { QueryClient } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import { expect, it } from 'vitest'
import {
  GetPostResponseSchema,
  ListPostsResponseSchema,
  PostSchema,
  type GetPostResponse,
} from '@/shared/api'
import { createFakePostsTransport, type FakePostsOptions } from '@/test/posts'
import { withProviders } from '@/test/session'
import { getPostQueryKey, listPostsQueryKey } from './post-queries'
import { useSavePublishedUrl } from './useSavePublishedUrl'

const FINALIZED = {
  slug: 'post',
  status: 'finalized',
  finalizedRevision: 1n,
  contentRevision: 1n,
}

function setup(options: FakePostsOptions = {}) {
  const transport = createFakePostsTransport({ posts: [FINALIZED], ...options })
  const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
  const postKey = getPostQueryKey(transport, 'post')
  const listKey = listPostsQueryKey(transport)
  queryClient.setQueryData(
    postKey,
    create(GetPostResponseSchema, {
      post: create(PostSchema, { slug: 'post', status: 'finalized' }),
    }),
  )
  queryClient.setQueryData(listKey, create(ListPostsResponseSchema, {}))
  const view = renderHook(() => useSavePublishedUrl(), {
    wrapper: withProviders(transport, queryClient),
  })
  return { ...view, transport, queryClient, postKey, listKey }
}

// The answer IS the post, so the detail entry is replaced — never refetched — and the list's
// badge is marked stale.
it('replaces the GetPost entry with the answer and invalidates the list', async () => {
  const { result, queryClient, postKey, listKey } = setup()

  let saved: Awaited<ReturnType<typeof result.current.save>> | undefined
  await act(async () => {
    saved = await result.current.save('post', 'https://m.blog.naver.com/alice/1')
  })

  expect(saved?.status).toBe('published')
  expect(saved?.publishedUrl).toBe('https://blog.naver.com/alice/1')
  const cached = queryClient.getQueryData<GetPostResponse>(postKey)
  expect(cached?.post?.status).toBe('published')
  expect(cached?.post?.publishedUrl).toBe('https://blog.naver.com/alice/1')
  expect(queryClient.getQueryState(postKey)?.isInvalidated).toBe(false)
  expect(queryClient.getQueryState(listKey)?.isInvalidated).toBe(true)
})

// QUAL-3: each changes which posts are 발행됨, and so the window M2 and the aggregate read.
it('marks every quality query stale after a paste, a replace and a clear', async () => {
  const { result, transport, queryClient } = setup()
  const qualityKey = ['quality', transport, 'alice', 'measurement', 'post-a', '1']
  for (const url of ['https://blog.naver.com/alice/1', 'https://blog.naver.com/alice/2', '']) {
    queryClient.setQueryData(qualityKey, {})
    expect(queryClient.getQueryState(qualityKey)?.isInvalidated).toBe(false)
    await act(async () => {
      await result.current.save('post', url)
    })
    await waitFor(() => expect(queryClient.getQueryState(qualityKey)?.isInvalidated).toBe(true))
  }
})

it('exposes a refusal as an AppFailure and leaves the cache alone', async () => {
  const { result, queryClient, postKey } = setup({ publishedUrlBusy: true })

  await act(async () => {
    await expect(result.current.save('post', 'https://blog.naver.com/alice/1')).rejects.toBeTruthy()
  })

  await waitFor(() => expect(result.current.failure).toEqual({ reason: 'POST_BUSY', params: {} }))
  expect(queryClient.getQueryData<GetPostResponse>(postKey)?.post?.status).toBe('finalized')
  act(() => result.current.reset())
  await waitFor(() => expect(result.current.failure).toBeUndefined())
})
