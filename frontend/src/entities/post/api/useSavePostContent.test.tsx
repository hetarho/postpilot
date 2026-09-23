import { create } from '@bufbuild/protobuf'
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { QueryClient } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { expect, it } from 'vitest'
import {
  appFailureFromConnect,
  GetPostResponseSchema,
  ListPostsResponseSchema,
  PostContentSchema,
  PostSchema,
  PostService,
  SavePostContentResponseSchema,
} from '@/shared/api'
import { connectAppError } from '@/test/app-error'
import { withProviders } from '@/test/session'
import { getPostQueryKey, listPostsQueryKey } from './post-queries'
import { ContentRevisionConflictError, useSavePostContent } from './useSavePostContent'

function saveAgainst(cause: ConnectError) {
  const transport = createRouterTransport(({ rpc }) => {
    rpc(PostService.method.savePostContent, () => {
      throw cause
    })
  })
  const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
  const view = renderHook(() => useSavePostContent(), {
    wrapper: withProviders(transport, queryClient),
  })
  return view.result.current.save('post', create(PostContentSchema), 1n)
}

// QUAL-3: a saved block is a new revision, and every reading that counted the old text is stale.
it('marks every quality query stale after a save', async () => {
  const transport = createRouterTransport(({ rpc }) => {
    rpc(PostService.method.savePostContent, (req) =>
      create(SavePostContentResponseSchema, {
        post: create(PostSchema, { slug: req.slug, contentRevision: 2n, content: req.content }),
      }),
    )
  })
  const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
  const qualityKey = ['quality', transport, 'alice', 'measurement', 'post-a', '1']
  queryClient.setQueryData(qualityKey, {})
  const view = renderHook(() => useSavePostContent(), {
    wrapper: withProviders(transport, queryClient),
  })

  await expect(view.result.current.save('post-a', create(PostContentSchema), 1n)).resolves.toBe(2n)
  await waitFor(() => expect(queryClient.getQueryState(qualityKey)?.isInvalidated).toBe(true))
})

it('turns only POST_CONTENT_STALE into the local revision-conflict control state', async () => {
  await expect(
    saveAgainst(connectAppError('POST_CONTENT_STALE', Code.Aborted)),
  ).rejects.toBeInstanceOf(ContentRevisionConflictError)
})

it('keeps malformed Aborted failures generic instead of inferring a stale revision', async () => {
  const cause = await saveAgainst(new ConnectError('private transport prose', Code.Aborted)).catch(
    (error: unknown) => error,
  )

  expect(cause).not.toBeInstanceOf(ContentRevisionConflictError)
  expect(appFailureFromConnect(cause)).toEqual({ reason: 'UNKNOWN_FAILURE', params: {} })
})

// POST-86: published in another tab, the post's refetch reads it locked, which unmounts ②'s
// editor and its queue. The refusal itself still reaches the caller, so the queue's retry rule
// can read it.
it('refetches the post and the list when the post was published elsewhere', async () => {
  const transport = createRouterTransport(({ rpc }) => {
    rpc(PostService.method.savePostContent, () => {
      throw connectAppError('POST_PUBLISHED_LOCKED', Code.FailedPrecondition)
    })
  })
  const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
  const postKey = getPostQueryKey(transport, 'post')
  const listKey = listPostsQueryKey(transport)
  queryClient.setQueryData(
    postKey,
    create(GetPostResponseSchema, { post: create(PostSchema, { slug: 'post' }) }),
  )
  queryClient.setQueryData(listKey, create(ListPostsResponseSchema, {}))
  const view = renderHook(() => useSavePostContent(), {
    wrapper: withProviders(transport, queryClient),
  })

  const cause = await view.result.current
    .save('post', create(PostContentSchema), 1n)
    .catch((error: unknown) => error)

  expect(appFailureFromConnect(cause).reason).toBe('POST_PUBLISHED_LOCKED')
  await waitFor(() => expect(queryClient.getQueryState(postKey)?.isInvalidated).toBe(true))
  expect(queryClient.getQueryState(listKey)?.isInvalidated).toBe(true)
})
