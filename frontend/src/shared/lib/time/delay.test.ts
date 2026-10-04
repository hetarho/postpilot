import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { delay } from './delay'

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

it('leaves no abort listener behind once its wait is over', async () => {
  const controller = new AbortController()
  const added = vi.spyOn(controller.signal, 'addEventListener')
  const removed = vi.spyOn(controller.signal, 'removeEventListener')
  for (let wait = 0; wait < 10; wait++) {
    const waited = delay(2_000, controller.signal)
    await vi.advanceTimersByTimeAsync(2_000)
    await waited
  }
  expect(added).toHaveBeenCalledTimes(10)
  expect(removed).toHaveBeenCalledTimes(10)
  // Each removal names the very listener its wait added.
  for (let wait = 0; wait < 10; wait++)
    expect(removed.mock.calls[wait]![1]).toBe(added.mock.calls[wait]![1])
})

it('rejects with the abort reason the moment the signal aborts, and stops its timer', async () => {
  const controller = new AbortController()
  const waited = delay(2_000, controller.signal)
  const reason = new Error('left the page')
  controller.abort(reason)
  await expect(waited).rejects.toBe(reason)
  expect(vi.getTimerCount()).toBe(0)
})

it('rejects at once on a signal that has already aborted', async () => {
  const controller = new AbortController()
  controller.abort('gone')
  await expect(delay(2_000, controller.signal)).rejects.toBe('gone')
  expect(vi.getTimerCount()).toBe(0)
})

it('waits without a signal', async () => {
  let done = false
  void delay(500).then(() => (done = true))
  await vi.advanceTimersByTimeAsync(499)
  expect(done).toBe(false)
  await vi.advanceTimersByTimeAsync(1)
  expect(done).toBe(true)
})
