import { create } from '@bufbuild/protobuf'
import { Code } from '@connectrpc/connect'
import { afterEach, describe, expect, it, vi, type Mock } from 'vitest'
import { ContentRevisionConflictError, type ReplacementCandidate } from '@/entities/post'
import { BlockSchema, BlockType, PostContentSchema } from '@/shared/api'
import { AUTOSAVE_DEBOUNCE_MS, AUTOSAVE_RETRY_BASE_MS } from '@/shared/config'
import { connectAppError } from '@/test/app-error'
import {
  attachContentQueue,
  discardContentQueue,
  discardContentQueues,
  type ContentSnapshot,
  type SendContent,
} from './content-queue'

function snapshot(title: string, taken: ReplacementCandidate[] = []): ContentSnapshot {
  return {
    content: create(PostContentSchema, {
      title,
      blocks: [create(BlockSchema, { type: BlockType.TEXT, content: title })],
    }),
    taken,
  }
}

afterEach(() => {
  discardContentQueues()
  vi.useRealTimers()
})

describe('content save queue', () => {
  it('keeps one request in flight and sends only the newest pending snapshot', async () => {
    vi.useFakeTimers()
    const releases: Array<(revision: bigint) => void> = []
    const send = vi.fn((sent: ContentSnapshot, revision: bigint) => {
      void sent
      void revision
      return new Promise<{ revision: bigint; candidates: ReplacementCandidate[] }>((resolve) =>
        releases.push((next) => resolve({ revision: next, candidates: [] })),
      )
    })
    const handle = attachContentQueue({
      slug: 'post',
      revision: 1n,
      saved: snapshot('A'),
      candidates: [],
      send,
      onState: vi.fn(),
    })

    handle.queue(snapshot('B'))
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS)
    expect(send).toHaveBeenCalledTimes(1)
    expect(send.mock.calls[0]?.[0].content.title).toBe('B')
    expect(send.mock.calls[0]?.[1]).toBe(1n)

    handle.queue(snapshot('C'))
    handle.queue(snapshot('D'))
    expect(send).toHaveBeenCalledTimes(1)
    releases[0]?.(2n)
    await Promise.resolve()
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS)
    expect(send).toHaveBeenCalledTimes(2)
    expect(send.mock.calls[1]?.[0].content.title).toBe('D')
    expect(send.mock.calls[1]?.[1]).toBe(2n)

    releases[1]?.(3n)
    await Promise.resolve()
    await expect(handle.flush()).resolves.toBe(3n)
  })

  it('stops retry timers and rejects pending flushes when the session ends', async () => {
    vi.useFakeTimers()
    const send = vi.fn().mockRejectedValue(new Error('offline'))
    const handle = attachContentQueue({
      slug: 'post',
      revision: 1n,
      saved: snapshot('A'),
      candidates: [],
      send,
      onState: vi.fn(),
    })
    handle.queue(snapshot('B'))

    await expect(handle.flush()).rejects.toThrow('offline')
    expect(send).toHaveBeenCalledTimes(1)
    discardContentQueues()
    await vi.advanceTimersByTimeAsync(AUTOSAVE_RETRY_BASE_MS * 2)
    expect(send).toHaveBeenCalledTimes(1)
  })

  // The delete path's counterpart to the session-wide discard: one slug's queue ends, and
  // the other slugs keep retrying (tech/draft-autosave.md).
  it('discards one slug and leaves every other slug retrying', async () => {
    vi.useFakeTimers()
    const send = vi.fn().mockRejectedValue(new Error('offline'))
    const other = vi.fn().mockRejectedValue(new Error('offline'))
    const deleted = attachContentQueue({
      slug: 'gone',
      revision: 1n,
      saved: snapshot('A'),
      candidates: [],
      send,
      onState: vi.fn(),
    })
    const kept = attachContentQueue({
      slug: 'stays',
      revision: 1n,
      saved: snapshot('A'),
      candidates: [],
      send: other,
      onState: vi.fn(),
    })
    deleted.queue(snapshot('B'))
    kept.queue(snapshot('C'))

    await expect(deleted.flush()).rejects.toThrow('offline')
    await expect(kept.flush()).rejects.toThrow('offline')
    expect(send).toHaveBeenCalledTimes(1)

    discardContentQueue('gone')
    await vi.advanceTimersByTimeAsync(AUTOSAVE_RETRY_BASE_MS * 8)

    expect(send).toHaveBeenCalledTimes(1)
    expect(other.mock.calls.length).toBeGreaterThan(1)
  })

  it("rejects the discarded queue's pending flush with the delete reason", async () => {
    vi.useFakeTimers()
    const send = vi.fn(() => new Promise<never>(() => {}))
    const handle = attachContentQueue({
      slug: 'gone',
      revision: 1n,
      saved: snapshot('A'),
      candidates: [],
      send,
      onState: vi.fn(),
    })
    handle.queue(snapshot('B'))
    const flushed = handle.flush()
    discardContentQueue('gone')

    await expect(flushed).rejects.toThrow('post deleted')
  })

  it('surfaces an optimistic revision conflict without retrying it', async () => {
    vi.useFakeTimers()
    const states: string[] = []
    const send = vi.fn().mockRejectedValue(new ContentRevisionConflictError())
    const handle = attachContentQueue({
      slug: 'post',
      revision: 7n,
      saved: snapshot('A'),
      candidates: [],
      send,
      onState: (state) => states.push(state),
    })
    handle.queue(snapshot('B'))

    await expect(handle.flush()).rejects.toBeInstanceOf(ContentRevisionConflictError)
    expect(states.at(-1)).toBe('conflict')
    await vi.advanceTimersByTimeAsync(AUTOSAVE_RETRY_BASE_MS * 2)
    expect(send).toHaveBeenCalledTimes(1)
  })

  // Published in another tab (POST-86): every retry would be refused the same way, and the post's
  // refetch unmounts the editor this queue serves.
  it('does not retry a save refused because the post is published', async () => {
    vi.useFakeTimers()
    const locked = connectAppError('POST_PUBLISHED_LOCKED', Code.FailedPrecondition)
    const send = vi.fn().mockRejectedValue(locked)
    const handle = attachContentQueue({
      slug: 'post',
      revision: 7n,
      saved: snapshot('A'),
      candidates: [],
      send,
      onState: vi.fn(),
    })

    handle.queue(snapshot('B'))
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS)
    expect(send).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(AUTOSAVE_RETRY_BASE_MS * 16)
    expect(send).toHaveBeenCalledTimes(1)
  })

  it('keeps retrying an outage', async () => {
    vi.useFakeTimers()
    const send = vi.fn().mockRejectedValue(connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable))
    const handle = attachContentQueue({
      slug: 'post',
      revision: 7n,
      saved: snapshot('A'),
      candidates: [],
      send,
      onState: vi.fn(),
    })

    handle.queue(snapshot('B'))
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS + AUTOSAVE_RETRY_BASE_MS)
    expect(send).toHaveBeenCalledTimes(2)
  })
})

// POST-79: a take's candidate rides the save that carries its content, resolved to an index only
// when that save goes out, against the list at the revision it carries.
describe('taken candidates', () => {
  const candidate = (source: string, listIndex: number): ReplacementCandidate => ({
    surface: 'body',
    index: 0,
    source,
    phrases: [`${source}!`],
    listIndex,
  })
  const [a, b, c] = [candidate('a', 0), candidate('b', 1), candidate('c', 2)]
  type Answer = { revision: bigint; candidates: ReplacementCandidate[] }

  function attach(send: SendContent) {
    return attachContentQueue({
      slug: 'post',
      revision: 1n,
      saved: snapshot('A'),
      candidates: [a, b, c],
      send,
      onState: vi.fn(),
    })
  }
  const indicesOf = (send: Mock<SendContent>, call: number) => send.mock.calls[call]?.[2]
  const titleOf = (send: Mock<SendContent>, call: number) =>
    send.mock.calls[call]?.[0].content.title

  it('sends a take’s index with the save that carries its content', async () => {
    vi.useFakeTimers()
    const send = vi.fn<SendContent>(async (): Promise<Answer> => ({
      revision: 2n,
      candidates: [a, c],
    }))
    const handle = attach(send)

    handle.queue(snapshot('B', [b]))
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS)
    expect(send).toHaveBeenCalledTimes(1)
    expect(titleOf(send, 0)).toBe('B')
    expect(indicesOf(send, 0)).toEqual([1])
  })

  it('sends two takes and the typing between them as one save carrying both', async () => {
    vi.useFakeTimers()
    const send = vi.fn<SendContent>(async (): Promise<Answer> => ({
      revision: 2n,
      candidates: [a],
    }))
    const handle = attach(send)

    handle.queue(snapshot('B', [c]))
    handle.queue(snapshot('C', [b]))
    handle.queue(snapshot('D'))
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS)
    expect(send).toHaveBeenCalledTimes(1)
    expect(titleOf(send, 0)).toBe('D')
    expect(indicesOf(send, 0)).toEqual([1, 2])
  })

  it('sends a take made during another take’s save next, resolved against the answered list', async () => {
    vi.useFakeTimers()
    const releases: Array<(answer: Answer) => void> = []
    const send = vi.fn<SendContent>(() => new Promise<Answer>((resolve) => releases.push(resolve)))
    const handle = attach(send)

    handle.queue(snapshot('B', [b]))
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS)
    expect(indicesOf(send, 0)).toEqual([1])
    handle.queue(snapshot('C', [c]))
    // The server spent b: c is index 1 in what it answers.
    releases[0]?.({ revision: 2n, candidates: [candidate('a', 0), candidate('c', 1)] })
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS)
    expect(send).toHaveBeenCalledTimes(2)
    expect(titleOf(send, 1)).toBe('C')
    expect(send.mock.calls[1]?.[1]).toBe(2n)
    expect(indicesOf(send, 1)).toEqual([1])
  })

  it('sends typing made during a take’s save next, with no index', async () => {
    vi.useFakeTimers()
    const releases: Array<(answer: Answer) => void> = []
    const send = vi.fn<SendContent>(() => new Promise<Answer>((resolve) => releases.push(resolve)))
    const handle = attach(send)

    handle.queue(snapshot('B', [b]))
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS)
    handle.queue(snapshot('C'))
    releases[0]?.({ revision: 2n, candidates: [candidate('a', 0), candidate('c', 1)] })
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS)
    expect(send).toHaveBeenCalledTimes(2)
    expect(titleOf(send, 1)).toBe('C')
    expect(indicesOf(send, 1)).toEqual([])
  })

  it('keeps a failed save’s takes for the retry, even when typing merged into it', async () => {
    vi.useFakeTimers()
    const send = vi
      .fn<SendContent>()
      .mockRejectedValueOnce(connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable))
      .mockResolvedValue({ revision: 2n, candidates: [a, c] })
    const handle = attach(send)

    handle.queue(snapshot('B', [b]))
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS)
    expect(indicesOf(send, 0)).toEqual([1])
    handle.queue(snapshot('C'))
    await vi.advanceTimersByTimeAsync(AUTOSAVE_RETRY_BASE_MS)
    expect(send).toHaveBeenCalledTimes(2)
    expect(titleOf(send, 1)).toBe('C')
    expect(indicesOf(send, 1)).toEqual([1])
  })
})
