import { create } from '@bufbuild/protobuf'
import { createRouterTransport } from '@connectrpc/connect'
import { renderHook, cleanup, act } from '@testing-library/react'
import { afterEach, expect, it } from 'vitest'
import { createTestQueryClient, withProviders } from '@/test/session'
import { createConnectQueryKey } from '@connectrpc/connect-query'
import { PostService, ProviderService } from '@/shared/api'
import { mapWritingTest } from './mappers'
import { wireTest } from './fixtures.test-support'
import { writingTestsQueryKey } from './hooks'
import { useWritingTestOwnerRefresh } from './useWritingTestOwners'
afterEach(cleanup)
it('refreshes the canonical post and all same-owner history pages only after confirmed publication', async () => {
  const transport = createRouterTransport(() => {}),
    cache = createTestQueryClient()
  const postKey = createConnectQueryKey({
    schema: PostService.method.getPost,
    input: { slug: 'source' },
    transport,
    cardinality: 'finite',
  })
  const otherPostKey = createConnectQueryKey({
    schema: PostService.method.getPost,
    input: { slug: 'other' },
    transport,
    cardinality: 'finite',
  })
  const selectionKey = createConnectQueryKey({
    schema: ProviderService.method.getSelections,
    input: {},
    transport,
    cardinality: 'finite',
  })
  const aliceFirst = writingTestsQueryKey(transport, 'alice'),
    aliceNext = writingTestsQueryKey(transport, 'alice', { pageSize: 20, pageToken: 'next' }),
    bob = writingTestsQueryKey(transport, 'bob')
  for (const key of [postKey, otherPostKey, selectionKey, aliceFirst, aliceNext, bob])
    cache.setQueryData(key, { kept: true })
  const test = mapWritingTest(wireTest(2, true))
  test.sourcePostSlug = 'source'
  const publication = {
    id: 'receipt',
    testId: test.id,
    winnerCandidateId: test.winnerCandidateId,
    action: 'apply-output' as const,
    status: 'pending' as const,
    requestKey: 'key',
    targetId: 'source',
  }
  const hook = renderHook(() => useWritingTestOwnerRefresh('alice'), {
    wrapper: withProviders(transport, cache),
  })
  await act(async () => hook.result.current(test, publication))
  expect(cache.getQueryState(postKey)?.isInvalidated).toBe(false)
  await act(async () => hook.result.current(test, { ...publication, status: 'confirmed' }))
  for (const key of [postKey, aliceFirst, aliceNext])
    expect(cache.getQueryState(key)?.isInvalidated).toBe(true)
  for (const key of [otherPostKey, selectionKey, bob])
    expect(cache.getQueryState(key)?.isInvalidated).toBe(false)
})
it('refreshes selected models on confirmed adoption without publishing or touching post contents', async () => {
  const transport = createRouterTransport(() => {}),
    cache = createTestQueryClient()
  const selectionKey = createConnectQueryKey({
    schema: ProviderService.method.getSelections,
    input: {},
    transport,
    cardinality: 'finite',
  })
  const postKey = createConnectQueryKey({
    schema: PostService.method.getPost,
    input: { slug: 'source' },
    transport,
    cardinality: 'finite',
  })
  cache.setQueryData(selectionKey, create(ProviderService.method.getSelections.output, {}))
  cache.setQueryData(postKey, create(PostService.method.getPost.output, {}))
  const test = mapWritingTest(wireTest(2, true)),
    hook = renderHook(() => useWritingTestOwnerRefresh('alice'), {
      wrapper: withProviders(transport, cache),
    })
  await act(async () =>
    hook.result.current(test, {
      id: 'receipt',
      testId: test.id,
      winnerCandidateId: test.winnerCandidateId,
      action: 'adopt-model',
      status: 'confirmed',
      requestKey: 'key',
      targetId: 'write',
    }),
  )
  expect(cache.getQueryState(selectionKey)?.isInvalidated).toBe(true)
  expect(cache.getQueryState(postKey)?.isInvalidated).toBe(false)
})
