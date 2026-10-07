import { create } from '@bufbuild/protobuf'
import { createRouterTransport } from '@connectrpc/connect'
import { cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, expect, it } from 'vitest'
import { PostService } from '@/shared/api'
import { createTestQueryClient, withProviders } from '@/test/session'
import { useWritingTestSources } from './source'

afterEach(cleanup)
it('reads all owned summaries once without per-row details or AI and isolates owner changes', async () => {
  let activeOwner = 'alice'
  let listCalls = 0
  let detailCalls = 0
  const transport = createRouterTransport(({ rpc }) => {
    rpc(PostService.method.listPosts, (request) => {
      listCalls++
      expect(request.pageSize).toBe(0)
      return create(PostService.method.listPosts.output, {
        posts: [
          {
            slug: activeOwner,
            title: `${activeOwner} material`,
            status: 'draft',
            updatedAt: '2026-10-07T00:00:00Z',
          },
        ],
      })
    })
    rpc(PostService.method.getPost, () => {
      detailCalls++
      return create(PostService.method.getPost.output, {})
    })
  })
  const hook = renderHook(({ ownerId }) => useWritingTestSources(ownerId), {
    initialProps: { ownerId: '' },
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  expect(listCalls).toBe(0)
  hook.rerender({ ownerId: 'alice' })
  await waitFor(() => expect(hook.result.current.sources[0]?.name).toBe('alice material'))
  activeOwner = 'bob'
  hook.rerender({ ownerId: 'bob' })
  expect(hook.result.current.sources).toEqual([])
  await waitFor(() => expect(hook.result.current.sources[0]?.slug).toBe('bob'))
  expect(listCalls).toBe(2)
  expect(detailCalls).toBe(0)
})
