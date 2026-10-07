import { create } from '@bufbuild/protobuf'
import { createRouterTransport } from '@connectrpc/connect'
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, expect, it } from 'vitest'
import { WritingTestService as Service } from '@/shared/api'
import { createTestQueryClient, withProviders } from '@/test/session'
import { mapWritingTest } from './mappers'
import { wireTest } from './fixtures.test-support'
import {
  useWritingTest,
  useWritingTests,
  useWritingTestMutation,
  writingTestQueryKey,
  writingTestsQueryKey,
} from './hooks'
import type { WritingTest } from '../model/types'

afterEach(cleanup)
it('partitions owner and transport namespaces, and disabled views never create paid work', async () => {
  let reads = 0
  const transport = createRouterTransport(({ rpc }) => {
    rpc(Service.method.getWritingTest, () => {
      reads++
      return create(Service.method.getWritingTest.output, { test: wireTest() })
    })
    rpc(Service.method.listWritingTests, () => {
      reads++
      return create(Service.method.listWritingTests.output, { tests: [wireTest()] })
    })
  })
  const other = createRouterTransport(() => {})
  const cache = createTestQueryClient()
  const detail = renderHook(({ ownerId }) => useWritingTest(ownerId, 'test'), {
    initialProps: { ownerId: '' },
    wrapper: withProviders(transport, cache),
  })
  const history = renderHook(() => useWritingTests('alice'), {
    wrapper: withProviders(transport, cache),
  })
  await waitFor(() => expect(history.result.current.data?.tests).toHaveLength(1))
  expect(reads).toBe(1)
  detail.rerender({ ownerId: 'alice' })
  await waitFor(() => expect(detail.result.current.data?.id).toBe('test'))
  expect(reads).toBe(2)
  expect(writingTestQueryKey(transport, 'alice', 'test')).not.toEqual(
    writingTestQueryKey(transport, 'bob', 'test'),
  )
  expect(writingTestQueryKey(transport, 'alice', 'test')).not.toEqual(
    writingTestQueryKey(other, 'alice', 'test'),
  )
  expect(writingTestsQueryKey(transport, 'alice')).not.toEqual(
    writingTestsQueryKey(transport, 'bob'),
  )
})
it('keeps newer confirmed facts when an older delayed read completes', async () => {
  let release!: () => void
  const gate = new Promise<void>((resolve) => {
    release = resolve
  })
  const transport = createRouterTransport(({ rpc }) => {
    rpc(Service.method.getWritingTest, async () => {
      await gate
      return create(Service.method.getWritingTest.output, { test: wireTest() })
    })
  })
  const cache = createTestQueryClient()
  const hook = renderHook(() => useWritingTest('alice', 'test'), {
    wrapper: withProviders(transport, cache),
  })
  await waitFor(() => expect(hook.result.current.isFetching).toBe(true))
  const newer = mapWritingTest(wireTest())
  newer.revision = 9
  cache.setQueryData(writingTestQueryKey(transport, 'alice', 'test'), newer)
  await act(async () => {
    release()
    await gate
  })
  await waitFor(() => expect(hook.result.current.isFetching).toBe(false))
  expect(hook.result.current.data?.revision).toBe(9)
})
it('fences a late mutation cache write to its original owner after an account change', async () => {
  let release!: () => void
  const gate = new Promise<void>((resolve) => {
    release = resolve
  })
  const transport = createRouterTransport(({ rpc }) => {
    rpc(Service.method.cancelWritingTest, async () => {
      await gate
      return create(Service.method.cancelWritingTest.output, { test: wireTest() })
    })
  })
  const cache = createTestQueryClient()
  const hook = renderHook(({ ownerId }) => useWritingTestMutation(ownerId, 'cancel'), {
    initialProps: { ownerId: 'alice' },
    wrapper: withProviders(transport, cache),
  })
  let pending!: Promise<WritingTest>
  await act(async () => {
    pending = hook.result.current.mutateAsync({
      testId: 'test',
      expectedRevision: 3,
      requestKey: 'cancel-key',
    })
    await Promise.resolve()
  })
  hook.rerender({ ownerId: 'bob' })
  await act(async () => {
    release()
    await pending
  })
  expect(cache.getQueryData<WritingTest>(writingTestQueryKey(transport, 'alice', 'test'))?.id).toBe(
    'test',
  )
  expect(cache.getQueryData(writingTestQueryKey(transport, 'bob', 'test'))).toBeUndefined()
})
