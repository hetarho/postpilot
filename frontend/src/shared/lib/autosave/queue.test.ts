import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import {
  AUTOSAVE_DEBOUNCE_MS,
  AUTOSAVE_RETRY_BASE_MS,
  AUTOSAVE_RETRY_MAX_MS,
} from '@/shared/config'
import { createAutosaveQueue } from './queue'

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

/** A send whose every call can be settled by hand, so the flight window is a place to stand. */
function gate() {
  const calls: {
    draft: string
    resolve: (value?: unknown) => void
    reject: (e: unknown) => void
  }[] = []
  const send = vi.fn(
    (draft: string) =>
      new Promise<string>((resolve, reject) => {
        calls.push({ draft, resolve: () => resolve(draft), reject })
      }),
  )
  return { calls, send }
}

it('waits out the debounce, sends once, and reports the state on the way', async () => {
  const { calls, send } = gate()
  const queue = createAutosaveQueue<string, string>({ send })
  const seen: string[] = []
  queue.subscribe('a', () => seen.push(queue.state('a')))
  expect(queue.state('a')).toBe('idle')

  queue.queue('a', '첫')
  queue.queue('a', '둘')
  expect(queue.state('a')).toBe('dirty')
  expect(queue.peek('a')).toBe('둘')
  await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS - 1)
  expect(send).not.toHaveBeenCalled()
  await vi.advanceTimersByTimeAsync(1)
  // Only the newest draft goes out: the debounce is what folds the keystrokes together.
  expect(send).toHaveBeenCalledTimes(1)
  expect(calls[0]!.draft).toBe('둘')
  expect(queue.state('a')).toBe('saving')
  calls[0]!.resolve()
  await vi.advanceTimersByTimeAsync(0)
  expect(queue.state('a')).toBe('saved')
  expect(queue.result('a')).toBe('둘')
  expect(seen).toEqual(['dirty', 'dirty', 'saving', 'saved'])
})

it('queues a draft typed mid-flight instead of racing it, and folds it with merge', async () => {
  const { calls, send } = gate()
  const queue = createAutosaveQueue<string, string>({
    send,
    merge: (pending, next) => (pending ? `${pending}+${next}` : next),
  })
  queue.queue('a', '첫')
  await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS)
  queue.queue('a', '둘')
  queue.queue('a', '셋')
  // `merge` sees what the server is still owed — the draft in flight included, because a
  // request cannot be recalled and a retry carries whatever is pending when it fires.
  expect(queue.peek('a')).toBe('첫+둘+셋')
  expect(send).toHaveBeenCalledTimes(1)
  calls[0]!.resolve()
  await vi.advanceTimersByTimeAsync(0)
  // `follow: 'now'` (the default): what arrived during the flight goes out as it lands.
  expect(send).toHaveBeenCalledTimes(2)
  expect(calls[1]!.draft).toBe('첫+둘+셋')
})

it('lets a follow-up wait out another debounce when the feature asks for it', async () => {
  const { calls, send } = gate()
  const queue = createAutosaveQueue<string, string>({ send, follow: 'debounce' })
  queue.queue('a', '첫')
  await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS)
  queue.queue('a', '둘')
  calls[0]!.resolve()
  await vi.advanceTimersByTimeAsync(0)
  expect(send).toHaveBeenCalledTimes(1)
  await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS)
  expect(send).toHaveBeenCalledTimes(2)
})

it('retries a transient failure on a doubling backoff, capped', async () => {
  const send = vi.fn(async () => {
    throw new Error('offline')
  })
  const queue = createAutosaveQueue<string, void>({ send })
  queue.queue('a', '첫')
  await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS)
  expect(queue.state('a')).toBe('error')
  let delay = AUTOSAVE_RETRY_BASE_MS
  for (let attempt = 2; attempt <= 6; attempt += 1) {
    await vi.advanceTimersByTimeAsync(delay - 1)
    expect(send).toHaveBeenCalledTimes(attempt - 1)
    await vi.advanceTimersByTimeAsync(1)
    expect(send).toHaveBeenCalledTimes(attempt)
    delay = Math.min(delay * 2, AUTOSAVE_RETRY_MAX_MS)
  }
  // The pending draft is kept the whole time: it is what every retry carries.
  expect(queue.peek('a')).toBe('첫')
})

it('stops on a refusal the server will repeat, and tells everyone waiting', async () => {
  const refusal = new Error('refused')
  const send = vi.fn(async () => {
    throw refusal
  })
  const queue = createAutosaveQueue<string, void>({ send, retry: () => false })
  queue.queue('a', '첫')
  const flush = queue.flush('a')
  await expect(flush).rejects.toBe(refusal)
  expect(queue.state('a')).toBe('refused')
  expect(queue.failure('a')).toBe(refusal)
  await vi.advanceTimersByTimeAsync(AUTOSAVE_RETRY_MAX_MS)
  expect(send).toHaveBeenCalledTimes(1)
  // The next edit is a new attempt.
  queue.queue('a', '둘')
  expect(queue.state('a')).toBe('dirty')
  await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS)
  expect(send).toHaveBeenCalledTimes(2)
})

it('settles a flush only when the key runs dry, and fails fast on request', async () => {
  const { calls, send } = gate()
  const queue = createAutosaveQueue<string, string>({ send })
  queue.queue('a', '첫')
  const settled = vi.fn()
  const fast = vi.fn()
  void queue.flush('a').then(settled)
  void queue.flush('a', true).catch(fast)
  // A flush sends NOW rather than waiting out the debounce.
  await vi.advanceTimersByTimeAsync(0)
  expect(send).toHaveBeenCalledTimes(1)
  calls[0]!.reject(new Error('offline'))
  await vi.advanceTimersByTimeAsync(0)
  expect(fast).toHaveBeenCalledTimes(1)
  expect(settled).not.toHaveBeenCalled()
  await vi.advanceTimersByTimeAsync(AUTOSAVE_RETRY_BASE_MS)
  calls[1]!.resolve()
  await vi.advanceTimersByTimeAsync(0)
  expect(settled).toHaveBeenCalledTimes(1)
})

it('stops for good on a conflict, whatever is typed next', async () => {
  class Conflict extends Error {}
  const send = vi.fn(async () => {
    throw new Conflict()
  })
  const queue = createAutosaveQueue<string, void>({
    send,
    conflict: (error) => error instanceof Conflict,
  })
  queue.queue('a', '첫')
  await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS)
  expect(queue.state('a')).toBe('conflict')
  queue.queue('a', '둘')
  await vi.advanceTimersByTimeAsync(AUTOSAVE_RETRY_MAX_MS)
  expect(send).toHaveBeenCalledTimes(1)
  await expect(queue.flush('a')).rejects.toBeInstanceOf(Conflict)
})

it('drops a discarded key, keeps its subscribers, and leaves every other key alone', async () => {
  const { calls, send } = gate()
  const queue = createAutosaveQueue<string, string>({ send })
  const seen: string[] = []
  queue.subscribe('a', () => seen.push(queue.state('a')))
  queue.queue('a', '첫')
  queue.queue('b', '다른')
  await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS)
  const waited = vi.fn()
  void queue.flush('a').then(waited)
  queue.discard('a')
  await vi.advanceTimersByTimeAsync(0)
  expect(waited).toHaveBeenCalledTimes(1)
  expect(queue.state('a')).toBe('idle')
  expect(queue.peek('a')).toBeUndefined()
  // A request already out still lands; nothing it does may touch the registry again.
  calls[0]!.resolve()
  await vi.advanceTimersByTimeAsync(0)
  expect(queue.state('a')).toBe('idle')
  // The subscriber is still there for the next queue created under that key.
  queue.queue('a', '다시')
  expect(seen.at(-1)).toBe('dirty')
  expect(queue.state('b')).toBe('saving')
})

it('rejects the waiters of a key discarded with a reason', async () => {
  const { send } = gate()
  const queue = createAutosaveQueue<string, string>({ send })
  queue.queue('a', '첫')
  const flush = queue.flush('a')
  queue.discardAll(new Error('session ended'))
  await expect(flush).rejects.toThrow('session ended')
})

it('drops a draft that is already what the server holds', async () => {
  const { calls, send } = gate()
  const queue = createAutosaveQueue<string, string>({
    send,
    settled: (draft, sending, key) => draft === (sending ?? saved.get(key)),
  })
  const saved = new Map<string, string>()
  queue.queue('a', '첫')
  await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS)
  // Typed back to what the request now out is making the server hold.
  queue.queue('a', '첫')
  expect(queue.peek('a')).toBeUndefined()
  saved.set('a', '첫')
  calls[0]!.resolve()
  await vi.advanceTimersByTimeAsync(0)
  queue.queue('a', '첫')
  await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS)
  expect(send).toHaveBeenCalledTimes(1)
})
