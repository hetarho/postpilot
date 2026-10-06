import { create } from '@bufbuild/protobuf'
import { createRouterTransport } from '@connectrpc/connect'
import { renderHook, waitFor, cleanup, act } from '@testing-library/react'
import { afterEach, expect, it } from 'vitest'
import { ConfigurationAuthoringService as Service, ProtoConfigurationKind } from '@/shared/api'
import { createTestQueryClient, withProviders } from '@/test/session'
import {
  authoringLatestQueryKey,
  authoringSessionQueryKey,
  useAuthoringAPI,
  useLatestAuthoringSession,
  useAuthoringSession,
} from './hooks'
import type { AuthoringScope } from '../model/types'

afterEach(cleanup)
const scope: AuthoringScope = { ownerId: 'alice', kind: 'post-template', targetId: 'owned' }
it('partitions reads by transport, owner, kind and exact target without creating a session', async () => {
  const calls: Array<{ name: string; targetId: string }> = []
  const transport = createRouterTransport(({ rpc }) => {
    rpc(Service.method.getLatestAuthoringSession, (request) => {
      calls.push({ name: 'latest', targetId: request.targetId })
      return create(Service.method.getLatestAuthoringSession.output, {})
    })
  })
  const other = createRouterTransport(() => {})
  const cache = createTestQueryClient()
  const rendered = renderHook(() => useLatestAuthoringSession(scope), {
    wrapper: withProviders(transport, cache),
  })
  await waitFor(() => expect(rendered.result.current.data).toBeNull())
  expect(calls).toEqual([{ name: 'latest', targetId: 'owned' }])
  const key = authoringLatestQueryKey(transport, scope)
  for (const changed of [
    { ...scope, ownerId: 'bob' },
    { ...scope, targetId: '' },
    { ...scope, kind: 'video-template' as const },
  ])
    expect(authoringLatestQueryKey(transport, changed)).not.toEqual(key)
  expect(authoringLatestQueryKey(other, scope)).not.toEqual(key)
})
it('keeps newer session data when a late mutation returns an older revision', async () => {
  const transport = createRouterTransport(({ rpc }) => {
    rpc(Service.method.selectAuthoringCandidate, () =>
      create(Service.method.selectAuthoringCandidate.output, {
        session: {
          id: 'session',
          kind: ProtoConfigurationKind.POST_TEMPLATE,
          targetId: 'owned',
          revision: 2,
          phase: 'editing',
          selected: { id: 'candidate', name: '일상', body: '<write>본문</write>' },
        },
      }),
    )
  })
  const cache = createTestQueryClient()
  const key = authoringSessionQueryKey(transport, scope, 'session')
  cache.setQueryData(key, {
    id: 'session',
    kind: 'post-template',
    targetId: 'owned',
    revision: 8,
    phase: 'editing',
    selected: { id: 'newest', name: '최신초안', body: '<write>최신</write>' },
  })
  const hook = renderHook(() => useAuthoringAPI(scope), {
    wrapper: withProviders(transport, cache),
  })
  await hook.result.current.select('session', 1, 'candidate')
  expect(cache.getQueryData<{ revision: number }>(key)?.revision).toBe(8)
})

it('preserves a newer mutation while an older in-flight session read finishes', async () => {
  let release!: () => void
  const gate = new Promise<void>((resolve) => {
    release = resolve
  })
  const transport = createRouterTransport(({ rpc }) => {
    rpc(Service.method.getAuthoringSession, async () => {
      await gate
      return create(Service.method.getAuthoringSession.output, {
        session: {
          id: 'session',
          kind: ProtoConfigurationKind.POST_TEMPLATE,
          targetId: 'owned',
          revision: 2,
          phase: 'editing',
          selected: { id: 'old', name: '이전', body: '<write>이전</write>' },
        },
      })
    })
  })
  const cache = createTestQueryClient()
  const key = authoringSessionQueryKey(transport, scope, 'session')
  const hook = renderHook(() => useAuthoringSession(scope, 'session'), {
    wrapper: withProviders(transport, cache),
  })
  await waitFor(() => expect(hook.result.current.isFetching).toBe(true))
  cache.setQueryData(key, {
    id: 'session',
    kind: 'post-template',
    targetId: 'owned',
    revision: 8,
    phase: 'editing',
    selected: { id: 'latest', name: '최신', body: '<write>최신</write>' },
  })
  await act(async () => {
    release()
    await gate
  })
  await waitFor(() => expect(hook.result.current.isFetching).toBe(false))
  expect(hook.result.current.data?.revision).toBe(8)
  expect(cache.getQueryData<{ revision: number }>(key)?.revision).toBe(8)
})
