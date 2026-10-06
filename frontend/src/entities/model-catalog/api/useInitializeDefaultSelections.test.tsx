import { create } from '@bufbuild/protobuf'
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { QueryClient } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import { expect, it } from 'vitest'
import {
  type GetSelectionsResponse,
  GetSelectionsResponseSchema,
  ProviderService,
  SaveSelectionResponseSchema,
  Stage,
} from '@/shared/api'
import { withProviders } from '@/test/session'
import { getSelectionsQueryKey } from './catalog-mappers'
import { useInitializeDefaultSelections } from './useInitializeDefaultSelections'
import { useSaveSelection } from './useSaveSelection'
import { useSelections } from './useSelections'

const response = (modelId: string) =>
  create(GetSelectionsResponseSchema, {
    selections: [{ stage: Stage.WRITE, ref: { providerId: 'p', modelId } }],
  })
const testCache = () =>
  new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
const gate = <T,>() => {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => {
    resolve = done
  })
  return { promise, resolve }
}

it('prepares once for concurrent account observers and caches the actual server selection', async () => {
  const completion = gate<GetSelectionsResponse>()
  let calls = 0
  const transport = createRouterTransport(({ rpc }) => {
    rpc(ProviderService.method.initializeDefaultSelections, () => {
      calls++
      return completion.promise
    })
  })
  const cache = testCache()
  const view = renderHook(
    () => ({
      first: useInitializeDefaultSelections('alice'),
      second: useInitializeDefaultSelections('alice'),
    }),
    { wrapper: withProviders(transport, cache) },
  )
  expect(view.result.current.first.phase).toBe('checking')
  await waitFor(() => expect(calls).toBe(1))
  completion.resolve(response('server-manual-choice'))
  await waitFor(() =>
    expect(
      cache.getQueryData<GetSelectionsResponse>(getSelectionsQueryKey(transport))?.selections[0]
        ?.ref?.modelId,
    ).toBe('server-manual-choice'),
  )
  expect(view.result.current.first.phase).toBe('ready')
  expect(view.result.current.second.isPending).toBe(false)
  view.rerender()
  expect(calls).toBe(1)
})

it('does not initialize without an owner', async () => {
  let calls = 0
  const transport = createRouterTransport(({ rpc }) => {
    rpc(ProviderService.method.initializeDefaultSelections, () => {
      calls++
      return response('unexpected')
    })
  })
  const view = renderHook(() => useInitializeDefaultSelections(''), {
    wrapper: withProviders(transport, testCache()),
  })
  expect(view.result.current.isPending).toBe(false)
  expect(view.result.current.isError).toBe(false)
  expect(calls).toBe(0)
})

it('ignores a previous account completion after the observer changes owner', async () => {
  const alice = gate<GetSelectionsResponse>()
  const bob = gate<GetSelectionsResponse>()
  let calls = 0
  const transport = createRouterTransport(({ rpc }) => {
    rpc(ProviderService.method.initializeDefaultSelections, () => {
      calls++
      return calls === 1 ? alice.promise : bob.promise
    })
  })
  const cache = testCache()
  const view = renderHook(({ ownerId }) => useInitializeDefaultSelections(ownerId), {
    initialProps: { ownerId: 'alice' },
    wrapper: withProviders(transport, cache),
  })
  await waitFor(() => expect(calls).toBe(1))
  view.rerender({ ownerId: 'bob' })
  await waitFor(() => expect(calls).toBe(2))
  bob.resolve(response('bob-choice'))
  await waitFor(() => expect(view.result.current.phase).toBe('ready'))
  await act(async () => {
    alice.resolve(response('alice-late'))
    await alice.promise
  })
  expect(
    cache.getQueryData<GetSelectionsResponse>(getSelectionsQueryKey(transport))?.selections[0]?.ref
      ?.modelId,
  ).toBe('bob-choice')
})

it('cannot publish a response after its account observer unmounts', async () => {
  const completion = gate<GetSelectionsResponse>()
  let started = false
  const transport = createRouterTransport(({ rpc }) => {
    rpc(ProviderService.method.initializeDefaultSelections, () => {
      started = true
      return completion.promise
    })
  })
  const cache = testCache()
  const view = renderHook(() => useInitializeDefaultSelections('alice'), {
    wrapper: withProviders(transport, cache),
  })
  await waitFor(() => expect(started).toBe(true))
  view.unmount()
  await act(async () => {
    completion.resolve(response('late-choice'))
    await completion.promise
  })
  expect(cache.getQueryData(getSelectionsQueryKey(transport))).toBeUndefined()
})

it('preserves a manual save that finishes while a default response is in flight', async () => {
  const completion = gate<GetSelectionsResponse>()
  let started = false
  const transport = createRouterTransport(({ rpc }) => {
    rpc(ProviderService.method.initializeDefaultSelections, () => {
      started = true
      return completion.promise
    })
    rpc(ProviderService.method.saveSelection, (request) =>
      create(SaveSelectionResponseSchema, {
        selection: { stage: request.stage, ref: request.ref },
      }),
    )
  })
  const cache = testCache()
  cache.setQueryData(getSelectionsQueryKey(transport), response('original'))
  const view = renderHook(
    () => ({ initialize: useInitializeDefaultSelections('alice'), save: useSaveSelection() }),
    { wrapper: withProviders(transport, cache) },
  )
  await waitFor(() => expect(started).toBe(true))
  act(() => view.result.current.save.save('write', { providerId: 'p', modelId: 'new-manual' }))
  await waitFor(() =>
    expect(
      cache.getQueryData<GetSelectionsResponse>(getSelectionsQueryKey(transport))?.selections[0]
        ?.ref?.modelId,
    ).toBe('new-manual'),
  )
  completion.resolve(response('original'))
  await waitFor(() => expect(view.result.current.initialize.phase).toBe('ready'))
  expect(
    cache.getQueryData<GetSelectionsResponse>(getSelectionsQueryKey(transport))?.selections[0]?.ref
      ?.modelId,
  ).toBe('new-manual')
})

it('cancels an older read so its empty answer cannot erase prepared models', async () => {
  const oldRead = gate<GetSelectionsResponse>()
  const completion = gate<GetSelectionsResponse>()
  let reads = 0
  const transport = createRouterTransport(({ rpc }) => {
    rpc(ProviderService.method.initializeDefaultSelections, () => completion.promise)
    rpc(ProviderService.method.getSelections, () => {
      reads++
      return oldRead.promise
    })
  })
  const cache = testCache()
  const view = renderHook(
    () => ({ initialize: useInitializeDefaultSelections('alice'), selections: useSelections() }),
    { wrapper: withProviders(transport, cache) },
  )
  await waitFor(() => expect(reads).toBe(1))
  completion.resolve(response('prepared'))
  await waitFor(() =>
    expect(view.result.current.selections.selections.write?.ref.modelId).toBe('prepared'),
  )
  await act(async () => {
    oldRead.resolve(create(GetSelectionsResponseSchema, {}))
    await oldRead.promise
  })
  expect(view.result.current.selections.selections.write?.ref.modelId).toBe('prepared')
})

it('exposes a readable failure and retries without retaining provider prose', async () => {
  let calls = 0
  const transport = createRouterTransport(({ rpc }) => {
    rpc(ProviderService.method.initializeDefaultSelections, () => {
      calls++
      if (calls === 1) throw new ConnectError('private provider diagnostic', Code.Unavailable)
      return create(GetSelectionsResponseSchema, {})
    })
  })
  const view = renderHook(() => useInitializeDefaultSelections('alice'), {
    wrapper: withProviders(transport, testCache()),
  })
  await waitFor(() => expect(view.result.current.phase).toBe('failed'))
  expect(JSON.stringify(view.result.current.failure)).not.toContain('private provider diagnostic')
  act(() => view.result.current.retry())
  await waitFor(() => expect(view.result.current.phase).toBe('ready'))
  expect(view.result.current.failure).toBeUndefined()
  expect(calls).toBe(2)
})
