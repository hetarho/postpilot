import { describe, expect, it, vi } from 'vitest'
import { createActor, waitFor } from 'xstate'
import { HEADING_STYLE_REQUEST_MAX_CHARS } from '../config'
import { headingSuggestionsMachine } from './heading-suggestions-machine'

const request = {
  writeModel: { providerId: 'p', modelId: 'w' },
  targetKind: 'post' as const,
  targetPath: ['맛집'],
  sections: [{ role: '방문 계기', description: '방문한 이유' }],
}
const begin = { type: 'start' as const, ownerId: 'alice', signature: 'roles-1', request }

describe('heading suggestion actor', () => {
  it('starts only on an explicit owner event and ignores duplicate presses and stale job events', async () => {
    let resolve!: (id: string) => void
    const start = vi.fn(
      () =>
        new Promise<string>((done) => {
          resolve = done
        }),
    )
    const actor = createActor(headingSuggestionsMachine, {
      input: { ownerId: 'alice', start },
    }).start()
    expect(start).not.toHaveBeenCalled()
    actor.send({ ...begin, ownerId: 'bob' })
    expect(start).not.toHaveBeenCalled()
    actor.send(begin)
    actor.send(begin)
    expect(start).toHaveBeenCalledTimes(1)
    resolve('job-1')
    await waitFor(actor, (snapshot) => snapshot.matches('running'))
    actor.send({ type: 'terminal', jobId: 'older-job', status: 'done' })
    expect(actor.getSnapshot().matches('running')).toBe(true)
    actor.send({ type: 'terminal', jobId: 'job-1', status: 'done' })
    actor.send({ type: 'result', jobId: 'older-job', groups: [['다른', '이전', '결과']] })
    expect(actor.getSnapshot().matches('reading')).toBe(true)
    actor.send({ type: 'result', jobId: 'job-1', groups: [[' 첫째 ', '둘째', '셋째']] })
    expect(actor.getSnapshot().context.groups).toEqual([['첫째', '둘째', '셋째']])
    expect(start).toHaveBeenCalledTimes(1)
    actor.stop()
  })

  it('freezes the owner style request into the start and refuses one over the limit', async () => {
    const start = vi.fn(async () => 'job-styled')
    const actor = createActor(headingSuggestionsMachine, {
      input: { ownerId: 'alice', start },
    }).start()
    actor.send({
      ...begin,
      request: { ...request, styleRequest: '가'.repeat(HEADING_STYLE_REQUEST_MAX_CHARS + 1) },
    })
    expect(start).not.toHaveBeenCalled()
    actor.send({ ...begin, request: { ...request, styleRequest: '이모지를 넣어 줘' } })
    await waitFor(actor, (snapshot) => snapshot.matches('running'))
    expect(start).toHaveBeenCalledWith(
      expect.objectContaining({ styleRequest: '이모지를 넣어 줘' }),
    )
    actor.stop()
  })

  it('rejects a partial result as one failure without retaining any candidate', async () => {
    const actor = createActor(headingSuggestionsMachine, {
      input: { ownerId: 'alice', start: async () => 'job' },
    }).start()
    actor.send(begin)
    await waitFor(actor, (snapshot) => snapshot.matches('running'))
    actor.send({ type: 'terminal', jobId: 'job', status: 'done' })
    actor.send({ type: 'result', jobId: 'job', groups: [['하나', '둘']] })
    expect(actor.getSnapshot().matches('failed')).toBe(true)
    expect(actor.getSnapshot().context.groups).toEqual([])
    expect(actor.getSnapshot().context.failure?.reason).toBe('MODEL_OUTPUT_INVALID')
    actor.stop()
  })

  it('does not restore paid work or a late admission into a new owner actor', async () => {
    let resolve!: (id: string) => void
    const start = vi.fn(
      () =>
        new Promise<string>((done) => {
          resolve = done
        }),
    )
    const alice = createActor(headingSuggestionsMachine, {
      input: { ownerId: 'alice', start },
    }).start()
    alice.send(begin)
    alice.stop()
    const bob = createActor(headingSuggestionsMachine, { input: { ownerId: 'bob', start } }).start()
    resolve('alice-late-job')
    await Promise.resolve()
    expect(bob.getSnapshot().matches('idle')).toBe(true)
    expect(bob.getSnapshot().context.jobId).toBe('')
    bob.send(begin)
    expect(start).toHaveBeenCalledTimes(1)
    bob.stop()
  })
})

it('validates every clip heading against its own label bound before publishing any choices', async () => {
  const actor = createActor(headingSuggestionsMachine, {
    input: { ownerId: 'alice', start: async () => 'job', maxChars: 10 },
  }).start()
  actor.send({ ...begin, request: { ...request, targetKind: 'clip' } })
  await waitFor(actor, (snapshot) => snapshot.matches('running'))
  actor.send({ type: 'terminal', jobId: 'job', status: 'done' })
  actor.send({ type: 'result', jobId: 'job', groups: [['가'.repeat(11), '둘째', '셋째']] })
  expect(actor.getSnapshot().matches('failed')).toBe(true)
  expect(actor.getSnapshot().context.groups).toEqual([])
  actor.stop()
})
