import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import { POLL_INTERVAL_MS } from '@/shared/config'
import { createFakeJobsTransport } from '@/test/jobs'
import { createTestQueryClient, withProviders } from '@/test/session'
import { useJob } from './useJob'

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

async function tick(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms)
  })
}

it.each(['done', 'failed', 'cancelled'])(
  'polls every two seconds and stops after %s',
  async (status) => {
    const calls: string[] = []
    const transport = createFakeJobsTransport({
      calls,
      sequence: [
        { id: 'job-1', status: 'queued' },
        { id: 'job-1', status: 'running', stage: 'observe', progressDone: 1, progressTotal: 2 },
        { id: 'job-1', status, stage: 'write', progressDone: 1, progressTotal: 1 },
      ],
    })
    const view = renderHook(() => useJob('job-1'), {
      wrapper: withProviders(transport, createTestQueryClient()),
    })

    await tick(1)
    expect(calls).toHaveLength(1)
    expect(view.result.current.job?.status).toBe('queued')

    await tick(POLL_INTERVAL_MS)
    await tick(1)
    expect(calls).toHaveLength(2)
    expect(view.result.current.job?.progressDone).toBe(1)

    await tick(POLL_INTERVAL_MS)
    await tick(0)
    expect(calls).toHaveLength(3)
    expect(view.result.current.job?.status).toBe(status)

    await tick(POLL_INTERVAL_MS * 3)
    expect(calls).toHaveLength(3)
  },
)

it('invalidates the owner query when the job completes', async () => {
  const ownerKey = ['post', 'post-a'] as const
  const queryClient = createTestQueryClient()
  queryClient.setQueryData(ownerKey, { title: 'old' })
  const transport = createFakeJobsTransport({
    jobs: [{ id: 'job-1', status: 'done', stage: 'write' }],
  })
  renderHook(() => useJob('job-1', [ownerKey]), {
    wrapper: withProviders(transport, queryClient),
  })

  await tick(1)

  expect(queryClient.getQueryState(ownerKey)?.isInvalidated).toBe(true)
})

it('invalidates the owner query when the job fails so recoverable experiment state appears', async () => {
  const ownerKey = ['post', 'post-a'] as const
  const queryClient = createTestQueryClient()
  queryClient.setQueryData(ownerKey, { title: 'old' })
  const transport = createFakeJobsTransport({
    jobs: [{ id: 'job-1', status: 'failed', stage: 'compare_write' }],
  })
  renderHook(() => useJob('job-1', [ownerKey]), {
    wrapper: withProviders(transport, queryClient),
  })

  await tick(1)

  expect(queryClient.getQueryState(ownerKey)?.isInvalidated).toBe(true)
})

it('reopens and polls the same media job through waiting, execution, retry and cancellation', async () => {
  const calls: string[] = []
  const transport = createFakeJobsTransport({
    calls,
    sequence: [
      {
        id: 'media',
        kind: 'generate_clip',
        status: 'running',
        stage: 'prepare_wait',
        canCancel: true,
      },
      { id: 'media', kind: 'generate_clip', status: 'running', stage: 'prepare', canCancel: true },
      {
        id: 'media',
        kind: 'generate_clip',
        status: 'running',
        stage: 'prepare_retry',
        canCancel: true,
      },
      {
        id: 'media',
        kind: 'generate_clip',
        status: 'running',
        stage: 'analyze',
        progressDone: 2,
        progressTotal: 4,
        canCancel: true,
      },
      {
        id: 'media',
        kind: 'generate_clip',
        status: 'running',
        stage: 'analyze',
        progressDone: 2,
        progressTotal: 4,
        cancelRequestedAt: '2026-09-25',
      },
      {
        id: 'media',
        kind: 'generate_clip',
        status: 'cancelled',
        stage: 'analyze',
        progressDone: 2,
        progressTotal: 4,
      },
    ],
  })
  const first = renderHook(() => useJob('media'), {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  await tick(1)
  expect(first.result.current.job).toMatchObject({
    id: 'media',
    stage: 'prepare_wait',
    canCancel: true,
  })
  first.unmount()
  const reopened = renderHook(() => useJob('media'), {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  await tick(1)
  expect(reopened.result.current.job?.stage).toBe('prepare')
  await tick(POLL_INTERVAL_MS)
  await tick(1)
  expect(reopened.result.current.job).toMatchObject({
    id: 'media',
    stage: 'prepare_retry',
    canCancel: true,
  })
  await tick(POLL_INTERVAL_MS)
  await tick(1)
  expect(reopened.result.current.job).toMatchObject({ progressDone: 2, progressTotal: 4 })
  await tick(POLL_INTERVAL_MS)
  await tick(1)
  expect(reopened.result.current.job).toMatchObject({
    status: 'running',
    cancelRequestedAt: '2026-09-25',
  })
  await tick(POLL_INTERVAL_MS)
  await tick(1)
  expect(reopened.result.current.job?.status).toBe('cancelled')
  const terminalReads = calls.length
  await tick(POLL_INTERVAL_MS * 3)
  expect(calls).toHaveLength(terminalReads)
})
