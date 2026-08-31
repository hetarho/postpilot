import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { act, renderHook } from '@testing-library/react'
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
import { createTestQueryClient, withProviders } from '@/test/session'
import { usePosts } from './usePosts'

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

async function tick(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms)
  })
}

it('polls an active post until its durable experiment is ready for review', async () => {
  let calls = 0
  const transport = createRouterTransport(({ rpc }) => {
    rpc(PostService.method.listPosts, () => {
      calls += 1
      return create(ListPostsResponseSchema, {
        posts: [
          create(PostSummarySchema, {
            slug: 'post-a',
            targetLanguage: contentLanguageToProto('ko'),
            activeJob:
              calls === 1
                ? create(GenerationJobSchema, { id: 'job-1', status: 'running' })
                : undefined,
            pendingExperimentId: calls === 1 ? '' : 'experiment-1',
          }),
        ],
      })
    })
  })
  const view = renderHook(() => usePosts(), {
    wrapper: withProviders(transport, createTestQueryClient()),
  })

  await tick(1)
  expect(calls).toBe(1)
  expect(view.result.current.posts[0]?.activeJob?.id).toBe('job-1')

  await tick(POLL_INTERVAL_MS)
  await tick(1)
  expect(calls).toBe(2)
  expect(view.result.current.posts[0]?.pendingExperimentId).toBe('experiment-1')

  await tick(POLL_INTERVAL_MS * 2)
  expect(calls).toBe(2)
})
