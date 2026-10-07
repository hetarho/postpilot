import { create } from '@bufbuild/protobuf'
import { createRouterTransport, type Transport } from '@connectrpc/connect'
import { createConnectQueryKey } from '@connectrpc/connect-query'
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import type { QueryClient } from '@tanstack/react-query'
import { afterEach, describe, expect, it } from 'vitest'
import {
  AuthService,
  GetMeResponseSchema,
  WritingInspectionService as Service,
  emitUnauthenticated,
} from '@/shared/api'
import { createTestQueryClient, withProviders } from '@/test/session'
import { requestInspectionTargetKey, type RequestInspectionTarget } from '../model/types'
import { requestInspectionQueryKey, useRequestInspection } from './hooks'
import { capturedInspection, postTarget } from './fixtures.test-support'

afterEach(cleanup)
const selection = { stage: 'post-writing', status: 'captured' } as const

function session(cache: QueryClient, transport: Transport, ownerId: string) {
  const key = createConnectQueryKey({
    schema: AuthService.method.getMe,
    input: {},
    transport,
    cardinality: 'finite',
  })
  cache.setQueryData(key, create(GetMeResponseSchema, { user: { id: ownerId } }))
  return key
}

function readTransport(onRead: () => void = () => {}) {
  return createRouterTransport(({ rpc }) => {
    rpc(Service.method.getPostRequestInspection, () => {
      onRead()
      return create(Service.method.getPostRequestInspection.output, {
        inspection: capturedInspection(),
      })
    })
  })
}

describe('private request inspection cache', () => {
  it('requires actual authenticated ownership and an explicitly open view; repeated opens perform only reads', async () => {
    let reads = 0
    const transport = readTransport(() => reads++)
    const cache = createTestQueryClient()
    const hook = renderHook(({ enabled }) => useRequestInspection(postTarget, selection, enabled), {
      initialProps: { enabled: false },
      wrapper: withProviders(transport, cache),
    })
    expect(reads).toBe(0)
    hook.rerender({ enabled: true })
    expect(reads).toBe(0)
    await act(async () => {
      session(cache, transport, 'bob')
    })
    expect(reads).toBe(0)
    await act(async () => {
      session(cache, transport, 'alice')
    })
    await waitFor(() => expect(hook.result.current.data?.inspection.callId).toBe('job:write:1'))
    expect(reads).toBe(1)
    hook.rerender({ enabled: false })
    expect(hook.result.current.data).toBeUndefined()
    expect(
      cache.getQueryData(requestInspectionQueryKey(transport, postTarget, selection)),
    ).toBeUndefined()
    hook.rerender({ enabled: true })
    await waitFor(() => expect(hook.result.current.data?.inspection.status).toBe('captured'))
    expect(reads).toBe(2)
  })

  it('partitions every owner, material, result, authoring proposal and test contestant identity', () => {
    const transport = readTransport()
    const other = readTransport()
    const key = requestInspectionQueryKey(transport, postTarget, selection)
    for (const changed of [
      { ...postTarget, ownerId: 'bob' },
      { ...postTarget, sourceRevision: 'new-inputs' },
      { ...postTarget, resultRevision: 'new-result' },
      { ...postTarget, planRevision: 'new-plan' },
      { ...postTarget, contextKey: 'local-new-memo' },
    ])
      expect(requestInspectionQueryKey(transport, changed, selection)).not.toEqual(key)
    expect(requestInspectionQueryKey(other, postTarget, selection)).not.toEqual(key)
    expect(
      requestInspectionQueryKey(transport, postTarget, { ...selection, stage: 'post-observation' }),
    ).not.toEqual(key)
    expect(
      requestInspectionQueryKey(transport, postTarget, { ...selection, status: 'current' }),
    ).not.toEqual(key)
    const target: RequestInspectionTarget = {
      ownerId: 'alice',
      kind: 'authoring',
      sessionId: 'session',
      authoringKind: 'post-template',
      revision: 7,
      mode: 'refine',
      prompt: 'request',
      model: { providerId: 'fake', modelId: 'model' },
      candidateCount: 1,
    }
    for (const changed of [
      { ...target, revision: 8 },
      { ...target, sessionId: 'new-session' },
      { ...target, authoringKind: 'writing-voice' as const },
      { ...target, mode: 'recommend' as const },
      { ...target, prompt: 'new request' },
      { ...target, model: { providerId: 'other', modelId: 'model' } },
      { ...target, model: { providerId: 'fake', modelId: 'new-model' } },
      { ...target, candidateCount: 3 },
      { ...target, operationId: 'job' },
      { ...target, candidateId: 'candidate' },
    ])
      expect(requestInspectionTargetKey(changed)).not.toEqual(requestInspectionTargetKey(target))
    const test: RequestInspectionTarget = {
      ownerId: 'alice',
      kind: 'test',
      testId: 'test',
      candidateId: 'a',
      revision: 3,
      blind: true,
    }
    for (const changed of [
      { ...test, candidateId: 'b' },
      { ...test, revision: 4 },
      { ...test, blind: false },
      { ...test, payloadExpiresAt: '2026-10-09T00:00:00Z' },
    ])
      expect(requestInspectionTargetKey(changed)).not.toEqual(requestInspectionTargetKey(test))
    expect(requestInspectionTargetKey(null)).toBe('')
  })

  it('clears old result and source material before fetching a new host revision', async () => {
    let reads = 0
    const transport = readTransport(() => reads++)
    const cache = createTestQueryClient()
    session(cache, transport, 'alice')
    const hook = renderHook(({ target }) => useRequestInspection(target, selection), {
      initialProps: { target: postTarget },
      wrapper: withProviders(transport, cache),
    })
    await waitFor(() => expect(hook.result.current.data).toBeDefined())
    hook.rerender({
      target: { ...postTarget, sourceRevision: 'edited-inputs', resultRevision: 'edited-result' },
    })
    expect(hook.result.current.data).toBeUndefined()
    expect(
      cache.getQueryData(requestInspectionQueryKey(transport, postTarget, selection)),
    ).toBeUndefined()
    await waitFor(() => expect(hook.result.current.data).toBeDefined())
    expect(reads).toBe(2)
  })

  it.each(['close', 'remove', 'logout', 'owner-switch'] as const)(
    'rejects delayed private reads after %s and never repopulates removed cache',
    async (end) => {
      let release!: () => void
      const gate = new Promise<void>((resolve) => {
        release = resolve
      })
      let reads = 0
      const transport = createRouterTransport(({ rpc }) => {
        rpc(Service.method.getPostRequestInspection, async () => {
          reads++
          await gate
          return create(Service.method.getPostRequestInspection.output, {
            inspection: capturedInspection(),
          })
        })
      })
      const cache = createTestQueryClient()
      session(cache, transport, 'alice')
      const key = requestInspectionQueryKey(transport, postTarget, selection)
      const hook = renderHook(
        ({ enabled }) => useRequestInspection(postTarget, selection, enabled),
        {
          initialProps: { enabled: true },
          wrapper: withProviders(transport, cache),
        },
      )
      await waitFor(() => expect(reads).toBe(1))
      await act(async () => {
        if (end === 'close') hook.rerender({ enabled: false })
        if (end === 'remove') cache.removeQueries({ queryKey: key, exact: true })
        if (end === 'logout') cache.removeQueries()
        if (end === 'owner-switch') session(cache, transport, 'bob')
        release()
        await gate
      })
      await waitFor(() => expect(hook.result.current.data).toBeUndefined())
      expect(cache.getQueryData(key)).toBeUndefined()
      expect(reads).toBe(1)
    },
  )

  it('erases already-read private projections after a session expiry event', async () => {
    const transport = readTransport()
    const cache = createTestQueryClient()
    session(cache, transport, 'alice')
    const hook = renderHook(() => useRequestInspection(postTarget, selection), {
      wrapper: withProviders(transport, cache),
    })
    await waitFor(() => expect(hook.result.current.data).toBeDefined())
    await act(async () => {
      emitUnauthenticated()
    })
    expect(hook.result.current.data).toBeUndefined()
    expect(
      cache.getQueryData(requestInspectionQueryKey(transport, postTarget, selection)),
    ).toBeUndefined()
  })

  it('refuses expired or unknown test payload retention without trying to regenerate it', async () => {
    let reads = 0
    const transport = createRouterTransport(({ rpc }) => {
      rpc(Service.method.getWritingTestRequestInspection, () => {
        reads++
        return create(Service.method.getWritingTestRequestInspection.output, {
          inspection: capturedInspection(),
        })
      })
    })
    const cache = createTestQueryClient()
    session(cache, transport, 'alice')
    for (const payloadExpiresAt of ['2020-01-01T00:00:00Z', 'invalid-retention']) {
      const target: RequestInspectionTarget = {
        ownerId: 'alice',
        kind: 'test',
        testId: 'test',
        candidateId: 'a',
        revision: 3,
        blind: false,
        payloadExpiresAt,
      }
      const hook = renderHook(() => useRequestInspection(target, selection), {
        wrapper: withProviders(transport, cache),
      })
      expect(hook.result.current.data).toBeUndefined()
      expect(hook.result.current.isPending).toBe(false)
      hook.unmount()
    }
    expect(reads).toBe(0)
  })

  it('removes previously read test detail when its retained payload expires while the view remains open', async () => {
    let reads = 0
    const transport = createRouterTransport(({ rpc }) => {
      rpc(Service.method.getWritingTestRequestInspection, () => {
        reads++
        return create(Service.method.getWritingTestRequestInspection.output, {
          inspection: capturedInspection(),
        })
      })
    })
    const cache = createTestQueryClient()
    session(cache, transport, 'alice')
    const target: RequestInspectionTarget = {
      ownerId: 'alice',
      kind: 'test',
      testId: 'test',
      candidateId: 'a',
      revision: 3,
      blind: false,
      payloadExpiresAt: new Date(Date.now() + 200).toISOString(),
    }
    const hook = renderHook(() => useRequestInspection(target, selection), {
      wrapper: withProviders(transport, cache),
    })
    await waitFor(() => expect(hook.result.current.data?.inspection.status).toBe('captured'))
    await waitFor(() => expect(hook.result.current.scopeAvailable).toBe(false))
    expect(hook.result.current.data).toBeUndefined()
    expect(
      cache.getQueryData(requestInspectionQueryKey(transport, target, selection)),
    ).toBeUndefined()
    expect(reads).toBe(1)
  })
})
