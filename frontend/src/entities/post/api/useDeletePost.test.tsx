import { expect, it } from 'vitest'
import { act, renderHook, waitFor } from '@testing-library/react'
import { createRouterTransport } from '@connectrpc/connect'
import { createConnectQueryKey } from '@connectrpc/connect-query'
import { create } from '@bufbuild/protobuf'
import {
  DeletePostResponseSchema,
  GetPostResponseSchema,
  ListExperimentsResponseSchema,
  ListPostsResponseSchema,
  ModelExperimentService,
  PostSchema,
  PostService,
  PostSummarySchema,
  contentLanguageToProto,
} from '@/shared/api'
import { createTestQueryClient, withProviders } from '@/test/session'
import { getPostQueryKey, listPostsQueryKey } from './post-queries'
import { useDeletePost } from './useDeletePost'

function transportWith(deleted: string[]) {
  return createRouterTransport(({ rpc }) => {
    rpc(PostService.method.listPosts, () =>
      create(ListPostsResponseSchema, {
        posts: [
          create(PostSummarySchema, {
            slug: 'post-a',
            targetLanguage: contentLanguageToProto('ko'),
          }),
        ],
      }),
    )
    rpc(PostService.method.getPost, () =>
      create(GetPostResponseSchema, {
        post: create(PostSchema, { slug: 'post-a', targetLanguage: contentLanguageToProto('ko') }),
      }),
    )
    rpc(PostService.method.deletePost, (req) => {
      deleted.push(req.slug)
      return create(DeletePostResponseSchema, {})
    })
  })
}

it("invalidates the post list and drops the deleted post's own entry", async () => {
  const deleted: string[] = []
  const transport = transportWith(deleted)
  const queryClient = createTestQueryClient()
  const listKey = listPostsQueryKey(transport)
  const detailKey = getPostQueryKey(transport, 'post-a')
  // A real stage-filtered experiment list, not the prefix the hook invalidates: this is what
  // proves the prefix actually reaches the entries the screens registered.
  const experimentsKey = createConnectQueryKey({
    schema: ModelExperimentService.method.listExperiments,
    input: { stage: 2 },
    transport,
    cardinality: 'finite',
  })
  queryClient.setQueryData(listKey, create(ListPostsResponseSchema, {}))
  queryClient.setQueryData(detailKey, create(GetPostResponseSchema, {}))
  queryClient.setQueryData(experimentsKey, create(ListExperimentsResponseSchema, {}))

  const view = renderHook(() => useDeletePost(), {
    wrapper: withProviders(transport, queryClient),
  })
  await act(async () => {
    await view.result.current.remove('post-a')
  })

  expect(deleted).toEqual(['post-a'])
  await waitFor(() => {
    // Removed rather than invalidated: refetching a slug that is now a 404 would store an
    // error where a clean cache miss belongs.
    expect(queryClient.getQueryState(detailKey)).toBeUndefined()
  })
  expect(queryClient.getQueryState(listKey)?.isInvalidated).toBe(true)
  // The post's experiments survive detached (`model_experiments.post_slug` → NULL), so the
  // list that names them is stale.
  expect(queryClient.getQueryState(experimentsKey)?.isInvalidated).toBe(true)
})
