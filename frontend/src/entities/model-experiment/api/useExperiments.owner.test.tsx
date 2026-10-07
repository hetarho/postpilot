import { create } from '@bufbuild/protobuf'
import { createRouterTransport } from '@connectrpc/connect'
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, expect, it } from 'vitest'
import {
  ModelExperimentService as Service,
  ExperimentStatus,
  ExperimentSource,
  ExperimentOrigin,
  Stage,
} from '@/shared/api'
import { createTestQueryClient, withProviders } from '@/test/session'
import { ownedExperimentsQueryKey, useExperiment, useExperiments } from './useExperiments'

afterEach(cleanup)
const experiment = (id: string) => ({
  id,
  stage: Stage.WRITE,
  status: ExperimentStatus.COMPLETED,
  source: ExperimentSource.POST,
  origin: ExperimentOrigin.LAB,
})
it('optional owner namespace partitions retained legacy reads by owner and transport and disables empty owners', async () => {
  let activeOwner = 'alice'
  let calls = 0
  const transport = createRouterTransport(({ rpc }) => {
    rpc(Service.method.listExperiments, () => {
      calls++
      return create(Service.method.listExperiments.output, {
        experiments: [experiment(activeOwner)],
      })
    })
  })
  const cache = createTestQueryClient()
  const hook = renderHook(({ ownerId }) => useExperiments(undefined, undefined, ownerId), {
    initialProps: { ownerId: '' },
    wrapper: withProviders(transport, cache),
  })
  expect(calls).toBe(0)
  hook.rerender({ ownerId: 'alice' })
  await waitFor(() => expect(hook.result.current.experiments[0]?.id).toBe('alice'))
  activeOwner = 'bob'
  hook.rerender({ ownerId: 'bob' })
  expect(hook.result.current.experiments).toEqual([])
  await waitFor(() => expect(hook.result.current.experiments[0]?.id).toBe('bob'))
  expect(calls).toBe(2)
  expect(ownedExperimentsQueryKey(transport, 'alice')).not.toEqual(
    ownedExperimentsQueryKey(transport, 'bob'),
  )
  expect(ownedExperimentsQueryKey(transport, 'alice')).not.toEqual(
    ownedExperimentsQueryKey(
      createRouterTransport(() => {}),
      'alice',
    ),
  )
})
it('does not publish a late prior-owner detail into the next owner cache', async () => {
  let activeOwner = 'alice'
  let started = false
  let release!: () => void
  const gate = new Promise<void>((resolve) => {
    release = resolve
  })
  const transport = createRouterTransport(({ rpc }) => {
    rpc(Service.method.getExperiment, async () => {
      const captured = activeOwner
      if (captured === 'alice') {
        started = true
        await gate
      }
      return create(Service.method.getExperiment.output, { experiment: experiment(captured) })
    })
  })
  const hook = renderHook(({ ownerId }) => useExperiment('paid-result', ownerId), {
    initialProps: { ownerId: 'alice' },
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  await waitFor(() => expect(started).toBe(true))
  activeOwner = 'bob'
  hook.rerender({ ownerId: 'bob' })
  await waitFor(() => expect(hook.result.current.experiment?.id).toBe('bob'))
  await act(async () => {
    release()
    await gate
  })
  expect(hook.result.current.experiment?.id).toBe('bob')
})
