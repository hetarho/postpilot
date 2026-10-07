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
  OriginReviewSchema,
} from '@/shared/api'
import { connectAppError } from '@/test/app-error'
import { withProviders } from '@/test/session'
import { getPostQueryKey, listPostsQueryKey } from './post-queries'
import { ContentRevisionConflictError, useSavePostContent } from './useSavePostContent'

it('installs content, hash and aligned origins together, including clearing absent evidence', async () => {
  let evidence: ReturnType<typeof create<typeof OriginReviewSchema>> | undefined = create(
    OriginReviewSchema,
    { version: 1, result: { contentRevision: 2n, contentHash: 'new' } },
  )
  const transport = createRouterTransport(({ rpc }) => {
    rpc(PostService.method.savePostContent, (req) =>
      create(SavePostContentResponseSchema, {
        post: {
          slug: req.slug,
          content: req.content,
          contentRevision: 2n,
          contentHash: 'new',
          contentOrigins: evidence,
        },
      }),
    )
  })
  const queryClient = new QueryClient()
  const key = getPostQueryKey(transport, 'post')
  queryClient.setQueryData(
    key,
    create(GetPostResponseSchema, {
      post: {
        slug: 'post',
        contentRevision: 1n,
        contentHash: 'old',
        contentOrigins: create(OriginReviewSchema, { version: 1 }),
      },
    }),
  )
  const view = renderHook(() => useSavePostContent(), {
    wrapper: withProviders(transport, queryClient),
  })
  await view.result.current.save('post', create(PostContentSchema, { title: '새 문구' }), 1n)
  expect(queryClient.getQueryData(key)).toMatchObject({
    post: {
      content: { title: '새 문구' },
      contentRevision: 2n,
      contentHash: 'new',
      contentOrigins: evidence,
    },
  })
  evidence = undefined
  await view.result.current.save(
    'post',
    create(PostContentSchema, { title: '근거 없는 새 문구' }),
    2n,
  )
  expect(queryClient.getQueryData(key)).toMatchObject({
    post: { contentHash: 'new', contentOrigins: undefined },
  })
})

it.each([
  'newer result',
  'owner change',
  'withdrawn source',
  'same revision published',
  'same revision finalized',
] as const)('does not revive stale origins after %s overtakes a save', async (change) => {
  let finish!: () => void
  const pending = new Promise<void>((resolve) => {
    finish = resolve
  })
  const transport = createRouterTransport(({ rpc }) => {
    rpc(PostService.method.savePostContent, async (req) => {
      await pending
      return create(SavePostContentResponseSchema, {
        post: {
          slug: req.slug,
          status: 'review',
          content: req.content,
          contentRevision: 2n,
          inputRevision: 1n,
          contentHash: 'saved',
          contentOrigins: { version: 1, result: { contentRevision: 2n, contentHash: 'saved' } },
        },
      })
    })
  })
  const queryClient = new QueryClient()
  const key = getPostQueryKey(transport, 'post')
  queryClient.setQueryData(
    key,
    create(GetPostResponseSchema, {
      post: { slug: 'post', contentRevision: 1n, inputRevision: 1n, contentHash: 'original' },
    }),
  )
  const view = renderHook(() => useSavePostContent(), {
    wrapper: withProviders(transport, queryClient),
  })
  const saving = view.result.current.save(
    'post',
    create(PostContentSchema, { title: 'old owner draft' }),
    1n,
  )
  await waitFor(() => expect(view.result.current).toBeDefined())
  // Yield through onMutate before replacing or removing the owning query.
  await new Promise((resolve) => setTimeout(resolve, 0))
  if (change === 'owner change') queryClient.removeQueries()
  const newer = create(GetPostResponseSchema, {
    post: {
      slug: 'post',
      content: { title: 'current owner result' },
      contentRevision:
        change === 'newer result' ? 3n : change.startsWith('same revision') ? 2n : 1n,
      status:
        change === 'same revision published'
          ? 'published'
          : change === 'same revision finalized'
            ? 'finalized'
            : 'review',
      inputRevision: change === 'withdrawn source' ? 2n : 1n,
      contentHash: 'current',
    },
  })
  queryClient.setQueryData(key, newer)
  finish()
  await saving
  const cached = queryClient.getQueryData<typeof newer>(key)!
  if (change === 'withdrawn source') {
    expect(cached.post?.contentRevision).toBe(2n)
    expect(cached.post?.contentOrigins).toBeUndefined()
    expect(queryClient.getQueryState(key)?.isInvalidated).toBe(true)
  } else expect(cached).toEqual(newer)
})

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

  await expect(view.result.current.save('post-a', create(PostContentSchema), 1n)).resolves.toEqual(
    2n,
  )
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
