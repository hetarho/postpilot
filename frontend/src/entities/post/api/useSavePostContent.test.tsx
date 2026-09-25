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
  ProtoReplacementSurface,
  ReplacementCandidateSchema,
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

  await expect(view.result.current.save('post-a', create(PostContentSchema), 1n)).resolves.toEqual({
    revision: 2n,
    candidates: [],
  })
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

// POST-79: the taken indices go as given, and the answer's shorter list reaches the cached post
// at once, so a spent mark does not come back before a refetch.
it('sends the taken indices and patches the answer’s candidates into the cache', async () => {
  const sent: number[][] = []
  const remaining = create(ReplacementCandidateSchema, {
    surface: ProtoReplacementSurface.BODY,
    index: 0,
    source: '비가',
    phrases: ['빗방울이'],
  })
  const transport = createRouterTransport(({ rpc }) => {
    rpc(PostService.method.savePostContent, (req) => {
      sent.push([...req.takenCandidates])
      return create(SavePostContentResponseSchema, {
        post: create(PostSchema, {
          slug: req.slug,
          contentRevision: 3n,
          content: req.content,
          replacementCandidates: [remaining],
        }),
      })
    })
  })
  const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
  const postKey = getPostQueryKey(transport, 'post')
  queryClient.setQueryData(
    postKey,
    create(GetPostResponseSchema, {
      post: create(PostSchema, {
        slug: 'post',
        contentRevision: 2n,
        replacementCandidates: [
          create(ReplacementCandidateSchema, {
            surface: ProtoReplacementSurface.TITLE,
            source: '제주',
          }),
          remaining,
        ],
      }),
    }),
  )
  const view = renderHook(() => useSavePostContent(), {
    wrapper: withProviders(transport, queryClient),
  })

  const answer = await view.result.current.save('post', create(PostContentSchema), 2n, [0])
  expect(sent).toEqual([[0]])
  expect(answer.revision).toBe(3n)
  expect(answer.candidates.map((c) => [c.source, c.listIndex])).toEqual([['비가', 0]])
  const cached = queryClient.getQueryData<{
    post?: { replacementCandidates: { source: string }[] }
  }>(postKey)
  expect(cached?.post?.replacementCandidates.map((c) => c.source)).toEqual(['비가'])

  // Nothing taken sends an empty list.
  await view.result.current.save('post', create(PostContentSchema), 3n)
  expect(sent.at(-1)).toEqual([])
})
