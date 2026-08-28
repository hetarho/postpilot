import { create } from '@bufbuild/protobuf'
import { createRouterTransport } from '@connectrpc/connect'
import { QueryClient } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { expect, it } from 'vitest'
import { GetPostResponseSchema, ImageSchema, PostSchema, PostService } from '@/shared/api'
import { withProviders } from '@/test/session'
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
