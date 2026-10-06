import { create } from '@bufbuild/protobuf'
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { ConfigurationAuthoringService as Service, ProtoConfigurationKind } from '@/shared/api'
import { createTestQueryClient, withProviders } from '@/test/session'
import { useAuthoring } from './useAuthoring'

afterEach(cleanup)
const scope = { ownerId: 'alice', kind: 'post-guideline' as const }
const model = { providerId: 'p', modelId: 'writer' }
const artifact = {
  id: 'candidate',
  name: '방향',
  description: '친절한 방향',
  body: '실제로 겪은 일만 자연스럽게 써요.',
  titleArea: '',
}
const deferred = () => {
  let release!: () => void
  const promise = new Promise<void>((resolve) => {
    release = resolve
  })
  return { promise, release }
}
function fixture(
  options: {
    initial?: boolean
    createGate?: Promise<void>
    saveGate?: Promise<void>
    unknownStart?: boolean
    unpriced?: boolean
  } = {},
) {
  const wire = (patch: object = {}) =>
    create(Service.method.getAuthoringSession.output, {
      session: {
        id: 'session',
        kind: ProtoConfigurationKind.POST_GUIDELINE,
        revision: 1,
        phase: 'editing',
        selected: artifact,
        ...patch,
      },
    }).session!
  let current = options.initial ? wire() : undefined
  let unknown = !!options.unknownStart
  const starts: Array<{ requestId: string; expectedRevision: number; model: string }> = []
  const calls = { latest: 0, estimate: 0, create: 0, save: 0 }
  const transport = createRouterTransport(({ rpc }) => {
    rpc(Service.method.getLatestAuthoringSession, () => {
      calls.latest++
      return create(Service.method.getLatestAuthoringSession.output, { session: current })
    })
    rpc(Service.method.getAuthoringSession, () =>
      create(Service.method.getAuthoringSession.output, { session: current }),
    )
    rpc(Service.method.estimateAuthoringOperation, () => {
      calls.estimate++
      return create(Service.method.estimateAuthoringOperation.output, {
        free: false,
        credits: options.unpriced ? undefined : 7n,
      })
    })
    rpc(Service.method.createAuthoringSession, async () => {
      calls.create++
      await options.createGate
      current = wire({ revision: 0, phase: 'choosing', selected: undefined })
      return create(Service.method.createAuthoringSession.output, { session: current })
    })
    rpc(Service.method.startAuthoringOperation, (request) => {
      starts.push({
        requestId: request.requestId,
        expectedRevision: request.expectedRevision,
        model: request.writeModel!.modelId,
      })
      if (unknown) {
        unknown = false
        throw new ConnectError('unknown admission', Code.Unavailable)
      }
      current = wire({ revision: 2 })
      return create(Service.method.startAuthoringOperation.output, {
        jobId: 'job',
        session: current,
      })
    })
    rpc(Service.method.saveAuthoringSession, async () => {
      calls.save++
      await options.saveGate
      current = wire({
        revision: 3,
        phase: 'saved',
        saved: { kind: ProtoConfigurationKind.POST_GUIDELINE, id: 'saved', name: '방향' },
      })
      return create(Service.method.saveAuthoringSession.output, { session: current })
    })
  })
  return { transport, calls, starts }
}

it('performs reads only on entry and invokes one quote/create/start despite duplicate confirmations', async () => {
  const f = fixture()
  const hook = renderHook(() => useAuthoring(scope, {}), {
    wrapper: withProviders(f.transport, createTestQueryClient()),
  })
  await waitFor(() => expect(hook.result.current.state.phase).toBe('idle'))
  expect(f.calls).toEqual({ latest: 1, estimate: 0, create: 0, save: 0 })
  const selectedModel = { ...model }
  act(() => {
    hook.result.current.quote('recommend', selectedModel)
    hook.result.current.quote('recommend', selectedModel)
  })
  selectedModel.modelId = 'changed-later'
  await waitFor(() => expect(hook.result.current.state.phase).toBe('confirming'))
  act(() => {
    hook.result.current.confirm()
    hook.result.current.confirm()
  })
  await waitFor(() => expect(hook.result.current.state.phase).toBe('editing'))
  expect(f.calls.estimate).toBe(1)
  expect(f.calls.create).toBe(1)
  expect(f.starts).toHaveLength(1)
  expect(f.starts[0].model).toBe('writer')
})

it('never continues paid Start when a confirmed Create settles after the owner view unmounts', async () => {
  const gate = deferred()
  const f = fixture({ createGate: gate.promise })
  const hook = renderHook(() => useAuthoring(scope, {}), {
    wrapper: withProviders(f.transport, createTestQueryClient()),
  })
  await waitFor(() => expect(hook.result.current.state.phase).toBe('idle'))
  act(() => {
    hook.result.current.quote('recommend', model)
  })
  await waitFor(() => expect(hook.result.current.state.phase).toBe('confirming'))
  act(() => {
    hook.result.current.confirm()
  })
  await waitFor(() => expect(f.calls.create).toBe(1))
  hook.unmount()
  await act(async () => {
    gate.release()
    await gate.promise
    await Promise.resolve()
  })
  expect(f.starts).toHaveLength(0)
})

it('retries an uncertain admission using the same frozen request instead of requoting or creating again', async () => {
  const f = fixture({ unknownStart: true })
  const hook = renderHook(() => useAuthoring(scope, {}), {
    wrapper: withProviders(f.transport, createTestQueryClient()),
  })
  await waitFor(() => expect(hook.result.current.state.phase).toBe('idle'))
  act(() => {
    hook.result.current.quote('recommend', model)
  })
  await waitFor(() => expect(hook.result.current.state.phase).toBe('confirming'))
  act(() => {
    hook.result.current.confirm()
  })
  await waitFor(() => expect(hook.result.current.state.phase).toBe('failed'))
  await act(async () => {
    await hook.result.current.retryOperation()
  })
  await waitFor(() => expect(hook.result.current.state.phase).toBe('editing'))
  expect(f.starts).toHaveLength(2)
  expect(f.starts[0]).toEqual(f.starts[1])
  expect(f.calls.estimate).toBe(1)
  expect(f.calls.create).toBe(1)
})

it('discards an obsolete owner save result and never admits an unpriced confirmation', async () => {
  const gate = deferred()
  const f = fixture({ initial: true, saveGate: gate.promise })
  const onSaved = vi.fn()
  const hook = renderHook(() => useAuthoring(scope, { onSaved }), {
    wrapper: withProviders(f.transport, createTestQueryClient()),
  })
  await waitFor(() => expect(hook.result.current.state.phase).toBe('editing'))
  act(() => {
    hook.result.current.save(false)
    hook.result.current.save(false)
  })
  await waitFor(() => expect(f.calls.save).toBe(1))
  hook.unmount()
  const bob = fixture()
  const newOwner = renderHook(() => useAuthoring({ ...scope, ownerId: 'bob' }, { onSaved }), {
    wrapper: withProviders(bob.transport, createTestQueryClient()),
  })
  await waitFor(() => expect(newOwner.result.current.state.phase).toBe('idle'))
  await act(async () => {
    gate.release()
    await gate.promise
    await Promise.resolve()
  })
  expect(onSaved).not.toHaveBeenCalled()
  expect(newOwner.result.current.state.phase).toBe('idle')
  newOwner.unmount()
  const noPrice = fixture({ unpriced: true })
  const unpriced = renderHook(() => useAuthoring(scope, {}), {
    wrapper: withProviders(noPrice.transport, createTestQueryClient()),
  })
  await waitFor(() => expect(unpriced.result.current.state.phase).toBe('idle'))
  act(() => {
    unpriced.result.current.quote('recommend', model)
  })
  await waitFor(() => expect(unpriced.result.current.state.phase).toBe('failed'))
  act(() => {
    unpriced.result.current.confirm()
  })
  expect(noPrice.calls.create).toBe(0)
  expect(noPrice.starts).toHaveLength(0)
})
