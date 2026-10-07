import { expect, it } from 'vitest'
import { VideoFrameBudget } from './frame-budget'
it('waits for frame/byte capacity, drains fairly and releases idempotently', async () => {
  const budget = new VideoFrameBudget(2, 10),
    signal = new AbortController().signal
  const first = await budget.acquire(6, signal)
  let granted = false
  const second = budget.acquire(6, signal).then((release) => {
    granted = true
    return release
  })
  await Promise.resolve()
  expect(granted).toBe(false)
  first()
  first()
  const release = await second
  expect(budget.snapshot()).toMatchObject({
    liveFrames: 1,
    liveBytes: 6,
    peakFrames: 1,
    peakBytes: 6,
  })
  release()
  expect(budget.snapshot().liveBytes).toBe(0)
})
it('cancels queued leases and rejects an over-budget resource before allocation', async () => {
  const budget = new VideoFrameBudget(1, 10),
    controller = new AbortController()
  const release = await budget.acquire(10, controller.signal)
  const next = budget.acquire(1, controller.signal)
  controller.abort()
  await expect(next).rejects.toMatchObject({ name: 'AbortError' })
  await expect(budget.acquire(11, new AbortController().signal)).rejects.toThrow(
    'CLIP_SOURCE_MEMORY_LIMIT',
  )
  release()
  expect(budget.snapshot()).toMatchObject({ liveFrames: 0, liveBytes: 0, queued: 0 })
})
