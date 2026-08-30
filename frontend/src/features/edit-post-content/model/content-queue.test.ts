import { create } from '@bufbuild/protobuf'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ContentRevisionConflictError } from '@/entities/post'
import { BlockSchema, BlockType, PostContentSchema } from '@/shared/api'
import { AUTOSAVE_DEBOUNCE_MS, AUTOSAVE_RETRY_BASE_MS } from '@/shared/config'
import { attachContentQueue, discardContentQueues, type ContentSnapshot } from './content-queue'

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
