import { ConnectError, Code } from '@connectrpc/connect'
import { describe, expect, it, vi } from 'vitest'
import { createProjectSaveQueue } from './project-save-queue'

function conflicted(conflictAs: 'error' | 'refused') {
  const queue = createProjectSaveQueue<string, () => Promise<void>>({
    conflictAs,
    debounceMs: 10,
    run: (send) => send(),
    retry: () => false,
  })
  return queue
}

describe('createProjectSaveQueue', () => {
  it('reports the failure of a refused send, the same object on every read', async () => {
    const queue = conflicted('refused')
    const error = new ConnectError('nope', Code.InvalidArgument)
    queue.queue('p', 'a', () => Promise.reject(error))
    await expect(queue.flush('p', true)).rejects.toBe(error)
    const first = queue.failure('p')
    expect(first).toBeDefined()
    expect(queue.failure('p')).toBe(first)
    queue.discardAll()
    expect(queue.failure('p')).toBeUndefined()
  })

  it('sends the newest sender and drops it with the queue', async () => {
    const queue = conflicted('error')
    const first = vi.fn(() => Promise.resolve())
    const second = vi.fn(() => Promise.resolve())
    queue.queue('p', 'a', first)
    queue.queue('p', 'b', second)
    expect(queue.peek('p')).toBe('b')
    await queue.flush('p')
    expect(first).not.toHaveBeenCalled()
    expect(second).toHaveBeenCalledTimes(1)
    expect(queue.state('p')).toBe('saved')
  })

  it.each(['error', 'refused'] as const)('maps a conflict to %s', async (conflictAs) => {
    const queue = createProjectSaveQueue<string, () => Promise<void>>({
      conflictAs,
      debounceMs: 10,
      run: (send) => send(),
      conflict: () => true,
    })
    const error = new ConnectError('stale', Code.Aborted)
    queue.queue('p', 'a', () => Promise.reject(error))
    await expect(queue.flush('p', true)).rejects.toBe(error)
    expect(queue.state('p')).toBe(conflictAs)
    // A conflict is not a failure to show: the snapshot stays empty, as it always has.
    expect(queue.failure('p')).toBeUndefined()
  })
})
