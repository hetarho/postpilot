import { create } from '@bufbuild/protobuf'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ContentRevisionConflictError } from '@/entities/post'
import { BlockSchema, BlockType, PostContentSchema } from '@/shared/api'
import { AUTOSAVE_DEBOUNCE_MS, AUTOSAVE_RETRY_BASE_MS } from '@/shared/config'
import {
  attachContentQueue,
  discardContentQueue,
  discardContentQueues,
  type ContentSnapshot,
} from './content-queue'

function snapshot(title: string): ContentSnapshot {
  return {
    content: create(PostContentSchema, {
      title,
      blocks: [create(BlockSchema, { type: BlockType.TEXT, content: title })],
    }),
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
      return new Promise<bigint>((resolve) => releases.push(resolve))
    })
    const handle = attachContentQueue({
      slug: 'post',
      revision: 1n,
      saved: snapshot('A'),
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
      send,
      onState: vi.fn(),
    })
    const kept = attachContentQueue({
      slug: 'stays',
      revision: 1n,
      saved: snapshot('A'),
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
    const send = vi.fn(() => new Promise<bigint>(() => {}))
    const handle = attachContentQueue({
      slug: 'gone',
      revision: 1n,
      saved: snapshot('A'),
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
      send,
      onState: (state) => states.push(state),
    })
    handle.queue(snapshot('B'))

    await expect(handle.flush()).rejects.toBeInstanceOf(ContentRevisionConflictError)
    expect(states.at(-1)).toBe('conflict')
    await vi.advanceTimersByTimeAsync(AUTOSAVE_RETRY_BASE_MS * 2)
    expect(send).toHaveBeenCalledTimes(1)
  })
})
