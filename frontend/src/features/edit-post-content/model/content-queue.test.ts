import { create } from '@bufbuild/protobuf'
import { Code } from '@connectrpc/connect'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ContentRevisionConflictError } from '@/entities/post'
import { BlockSchema, BlockType, PostContentSchema } from '@/shared/api'
import { AUTOSAVE_DEBOUNCE_MS, AUTOSAVE_RETRY_BASE_MS } from '@/shared/config'
import { connectAppError } from '@/test/app-error'
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
      machineBaselineRevision: 1n,
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

  it('starts from a new AI result when the old editor releases in the same commit', async () => {
    const send = vi.fn(async (_sent: ContentSnapshot, revision: bigint) => revision + 1n)
    const old = attachContentQueue({
      slug: 'post',
      revision: 1n,
      machineBaselineRevision: 1n,
      saved: snapshot('old AI result'),
      send,
      onState: vi.fn(),
    })

    // React runs the old effect cleanup immediately before the new editor's layout effect.
    // release() deletes the attachment only after a promise resolves, so this attach used to
    // inherit revision 1 and the old saved content despite receiving the AI's revision 2.
    old.release()
    const fresh = attachContentQueue({
      slug: 'post',
      revision: 2n,
      machineBaselineRevision: 2n,
      saved: snapshot('new AI result'),
      send,
      onState: vi.fn(),
    })
    await Promise.resolve()
    fresh.queue(snapshot('new AI result'))
    await expect(fresh.flush()).resolves.toBe(2n)
    expect(send).not.toHaveBeenCalled()

    fresh.queue(snapshot('owner edit'))
    await expect(fresh.flush()).resolves.toBe(3n)
    expect(send).toHaveBeenCalledWith(snapshot('owner edit'), 2n)
  })

  it('keeps a normal content save on the same machine baseline and uses its answered revision', async () => {
    let finishFirst!: (revision: bigint) => void
    const firstSend = vi.fn(() => new Promise<bigint>((resolve) => (finishFirst = resolve)))
    const old = attachContentQueue({
      slug: 'post',
      revision: 1n,
      machineBaselineRevision: 1n,
      saved: snapshot('AI result'),
      send: firstSend,
      onState: vi.fn(),
    })
    old.queue(snapshot('first edit'))
    const firstFlush = old.flush()
    const nextSend = vi.fn(async (_sent: ContentSnapshot, revision: bigint) => revision + 1n)
    const sameSession = attachContentQueue({
      slug: 'post',
      revision: 2n,
      machineBaselineRevision: 1n,
      saved: snapshot('first edit'),
      send: nextSend,
      onState: vi.fn(),
    })
    sameSession.queue(snapshot('second edit'))
    finishFirst(2n)
    await expect(firstFlush).resolves.toBe(3n)
    expect(nextSend).toHaveBeenCalledWith(snapshot('second edit'), 2n)
  })

  it('refuses a new AI baseline while an old edit is in flight without rebasing that edit', async () => {
    let finishOld!: (revision: bigint) => void
    const oldSend = vi.fn(() => new Promise<bigint>((resolve) => (finishOld = resolve)))
    const old = attachContentQueue({
      slug: 'post',
      revision: 1n,
      machineBaselineRevision: 1n,
      saved: snapshot('old AI result'),
      send: oldSend,
      onState: vi.fn(),
    })
    old.queue(snapshot('unsaved owner edit'))
    const oldFlush = old.flush()
    old.release()

    const newSend = vi.fn(async (_sent: ContentSnapshot, revision: bigint) => revision + 1n)
    const blocked = attachContentQueue({
      slug: 'post',
      revision: 2n,
      machineBaselineRevision: 2n,
      saved: snapshot('new AI result'),
      send: newSend,
      onState: vi.fn(),
    })
    expect(blocked.state()).toBe('conflict')
    blocked.queue(snapshot('new owner edit'))
    await expect(blocked.flush()).rejects.toBeInstanceOf(ContentRevisionConflictError)
    expect(newSend).not.toHaveBeenCalled()
    expect(oldSend).toHaveBeenCalledWith(snapshot('unsaved owner edit'), 1n)

    // Even if the old request answers after the machine write, it may not move the new
    // editor's revision or clear its explicit conflict. Reload is the recovery path.
    finishOld(2n)
    await expect(oldFlush).resolves.toBe(2n)
    expect(blocked.state()).toBe('conflict')
    await expect(blocked.flush()).rejects.toBeInstanceOf(ContentRevisionConflictError)
  })

  it('stops retry timers and rejects pending flushes when the session ends', async () => {
    vi.useFakeTimers()
    const send = vi.fn().mockRejectedValue(new Error('offline'))
    const handle = attachContentQueue({
      slug: 'post',
      revision: 1n,
      machineBaselineRevision: 1n,
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
  // the other slugs keep retrying (POST-12).
  it('discards one slug and leaves every other slug retrying', async () => {
    vi.useFakeTimers()
    const send = vi.fn().mockRejectedValue(new Error('offline'))
    const other = vi.fn().mockRejectedValue(new Error('offline'))
    const deleted = attachContentQueue({
      slug: 'gone',
      revision: 1n,
      machineBaselineRevision: 1n,
      saved: snapshot('A'),
      send,
      onState: vi.fn(),
    })
    const kept = attachContentQueue({
      slug: 'stays',
      revision: 1n,
      machineBaselineRevision: 1n,
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
      machineBaselineRevision: 1n,
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
      machineBaselineRevision: 7n,
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

  // Published in another tab (POST-86): every retry would be refused the same way, and the post's
  // refetch unmounts the editor this queue serves.
  it('does not retry a save refused because the post is published', async () => {
    vi.useFakeTimers()
    const locked = connectAppError('POST_PUBLISHED_LOCKED', Code.FailedPrecondition)
    const send = vi.fn().mockRejectedValue(locked)
    const handle = attachContentQueue({
      slug: 'post',
      revision: 7n,
      machineBaselineRevision: 7n,
      saved: snapshot('A'),
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
      machineBaselineRevision: 7n,
      saved: snapshot('A'),
      send,
      onState: vi.fn(),
    })

    handle.queue(snapshot('B'))
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS + AUTOSAVE_RETRY_BASE_MS)
    expect(send).toHaveBeenCalledTimes(2)
  })
})
