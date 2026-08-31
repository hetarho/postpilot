import { create } from '@bufbuild/protobuf'
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { QueryClient } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { expect, it } from 'vitest'
import {
  contentLanguageToProto,
  GetPostResponseSchema,
  ImageSchema,
  PostSchema,
  PostService,
} from '@/shared/api'
import { withProviders } from '@/test/session'
import { connectAppError } from '@/test/app-error'
import { getPostQueryKey } from './post-queries'
import { usePost } from './usePost'

it('refreshes image view URLs whenever a cached post is entered', async () => {
  const slug = '20260829-photo'
  let calls = 0
  const transport = createRouterTransport(({ rpc }) => {
    rpc(PostService.method.getPost, () => {
      calls += 1
      return create(GetPostResponseSchema, {
        post: create(PostSchema, {
          slug,
          targetLanguage: contentLanguageToProto('ko'),
          images: [
            create(ImageSchema, {
              id: 'image-1',
              filename: 'photo.jpg',
              viewUrl: 'https://storage.test/photo.jpg?signature=fresh',
            }),
          ],
        }),
      })
    })
  })
  // Model the production failure: autosave has just marked the post cache fresh while
  // preserving a URL that is no longer usable.
  const queryClient = new QueryClient({
    defaultOptions: { queries: { staleTime: Infinity, retry: false } },
  })
  queryClient.setQueryData(
    getPostQueryKey(transport, slug),
    create(GetPostResponseSchema, {
      post: create(PostSchema, {
        slug,
        targetLanguage: contentLanguageToProto('ko'),
        images: [
          create(ImageSchema, {
            id: 'image-1',
            filename: 'photo.jpg',
            viewUrl: 'blob:expired-preview',
          }),
        ],
      }),
    }),
  )

  const view = renderHook(() => usePost(slug), {
    wrapper: withProviders(transport, queryClient),
  })

  await waitFor(() => expect(calls).toBe(1))
  await waitFor(() =>
    expect(view.result.current.post?.images[0]?.viewUrl).toBe(
      'https://storage.test/photo.jpg?signature=fresh',
    ),
  )
})

it('exposes the required mount refresh even while cached post detail is available', async () => {
  const slug = '20260829-cached'
  let releaseRefresh: (() => void) | undefined
  const refreshGate = new Promise<void>((resolve) => {
    releaseRefresh = resolve
  })
  const transport = createRouterTransport(({ rpc }) => {
    rpc(PostService.method.getPost, async () => {
      await refreshGate
      return create(GetPostResponseSchema, {
        post: create(PostSchema, {
          slug,
          targetLength: 1_800,
          targetLanguage: contentLanguageToProto('ko'),
        }),
      })
    })
  })
  const queryClient = new QueryClient({
    defaultOptions: { queries: { staleTime: Infinity, retry: false } },
  })
  queryClient.setQueryData(
    getPostQueryKey(transport, slug),
    create(GetPostResponseSchema, {
      post: create(PostSchema, {
        slug,
        targetLength: 900,
        targetLanguage: contentLanguageToProto('ko'),
      }),
    }),
  )

  const view = renderHook(() => usePost(slug), {
    wrapper: withProviders(transport, queryClient),
  })

  expect(view.result.current.post?.targetLength).toBe(900)
  expect(view.result.current.isPending).toBe(false)
  expect(view.result.current.isFetching).toBe(true)

  releaseRefresh?.()
  await waitFor(() => expect(view.result.current.isFetching).toBe(false))
  expect(view.result.current.post?.targetLength).toBe(1_800)
})

it.each([
  ['POST_FORBIDDEN' as const, Code.PermissionDenied],
  ['POST_NOT_FOUND' as const, Code.NotFound],
])('classifies only the structured %s reason as a semantic load failure', async (reason, code) => {
  const transport = createRouterTransport(({ rpc }) => {
    rpc(PostService.method.getPost, () => {
      throw connectAppError(reason, code)
    })
  })
  const view = renderHook(() => usePost('missing'), {
    wrapper: withProviders(
      transport,
      new QueryClient({ defaultOptions: { queries: { retry: false } } }),
    ),
  })

  await waitFor(() => expect(view.result.current.failure?.reason).toBe(reason))
})

it('does not infer a product reason from a Connect status without AppErrorDetail', async () => {
  const transport = createRouterTransport(({ rpc }) => {
    rpc(PostService.method.getPost, () => {
      throw new ConnectError('private transport prose', Code.PermissionDenied)
    })
  })
  const view = renderHook(() => usePost('private'), {
    wrapper: withProviders(
      transport,
      new QueryClient({ defaultOptions: { queries: { retry: false } } }),
    ),
  })

  await waitFor(() =>
    expect(view.result.current.failure).toEqual({ reason: 'UNKNOWN_FAILURE', params: {} }),
  )
})
