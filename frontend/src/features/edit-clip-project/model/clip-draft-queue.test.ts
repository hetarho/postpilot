import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { Code, ConnectError } from '@connectrpc/connect'
import type { ClipProjectDraft } from '@/entities/clip-project'
import {
  clipDraftState,
  discardClipDraftQueues,
  flushClipDraft,
  peekPendingClipDraft,
  queueClipDraft,
  subscribeClipDraft,
} from './clip-draft-queue'

const draft = (title: string): ClipProjectDraft => ({
  title,
  videoTemplateId: 'template',
  ratio: 'vertical',
  targetDurationMs: 15000,
  answers: [],
  disclosure: 'ad' as const,
  cta: '' as const,
})

beforeEach(() => vi.useFakeTimers())
afterEach(() => {
  discardClipDraftQueues()
  vi.useRealTimers()
})

/** A send whose promises are settled by the test, so a flight can be held open deliberately. */
function controllable() {
  const sent: string[] = []
  const settle: { resolve: () => void; reject: (error: unknown) => void }[] = []
  const send = (value: ClipProjectDraft) => {
    sent.push(value.title)
    return new Promise<void>((resolve, reject) => settle.push({ resolve, reject }))
  }
  return { sent, settle, send }
}

describe('clip settings autosave queue', () => {
  it('saves once for a run of keystrokes', async () => {
    const { sent, settle, send } = controllable()
    queueClipDraft('clip', draft('제'), send)
    queueClipDraft('clip', draft('제주'), send)
    queueClipDraft('clip', draft('제주 여행'), send)
    expect(clipDraftState('clip')).toBe('dirty')
    expect(sent).toEqual([])

    await vi.advanceTimersByTimeAsync(1000)
    expect(sent).toEqual(['제주 여행'])
    expect(clipDraftState('clip')).toBe('saving')
    settle[0].resolve()
    await vi.advanceTimersByTimeAsync(0)
    expect(clipDraftState('clip')).toBe('saved')
    expect(peekPendingClipDraft('clip')).toBeUndefined()
  })

  it('queues a draft typed mid-flight instead of racing it', async () => {
    const { sent, settle, send } = controllable()
    queueClipDraft('clip', draft('첫'), send)
    await vi.advanceTimersByTimeAsync(1000)
    expect(sent).toEqual(['첫'])

    queueClipDraft('clip', draft('둘'), send)
    await vi.advanceTimersByTimeAsync(1000)
    // Still one request: the first has not answered, so nothing else may be in flight.
    expect(sent).toEqual(['첫'])

    settle[0].resolve()
    await vi.advanceTimersByTimeAsync(0)
    expect(sent).toEqual(['첫', '둘'])
    settle[1].resolve()
    await vi.advanceTimersByTimeAsync(0)
    expect(clipDraftState('clip')).toBe('saved')
  })

  it('retries a transient failure with a doubling backoff', async () => {
    const { sent, settle, send } = controllable()
    queueClipDraft('clip', draft('제주'), send)
    await vi.advanceTimersByTimeAsync(1000)
    settle[0].reject(new ConnectError('offline', Code.Unavailable))
    await vi.advanceTimersByTimeAsync(0)
    expect(clipDraftState('clip')).toBe('error')

    await vi.advanceTimersByTimeAsync(1000)
    expect(sent).toHaveLength(2)
    settle[1].reject(new ConnectError('offline', Code.Unavailable))
    await vi.advanceTimersByTimeAsync(0)
    // The second wait is longer than the first, so a dead connection is not hammered.
    await vi.advanceTimersByTimeAsync(1000)
    expect(sent).toHaveLength(2)
    await vi.advanceTimersByTimeAsync(1000)
    expect(sent).toHaveLength(3)

    settle[2].resolve()
    await vi.advanceTimersByTimeAsync(0)
    expect(clipDraftState('clip')).toBe('saved')
  })

  it('does not retry a refusal the server will repeat', async () => {
    const { sent, settle, send } = controllable()
    queueClipDraft('clip', draft('제주'), send)
    await vi.advanceTimersByTimeAsync(1000)
    settle[0].reject(new ConnectError('busy', Code.FailedPrecondition))
    await vi.advanceTimersByTimeAsync(0)
    expect(clipDraftState('clip')).toBe('error')

    await vi.advanceTimersByTimeAsync(60_000)
    expect(sent).toHaveLength(1)
    // The owner's next edit starts it again rather than a timer doing it.
    queueClipDraft('clip', draft('제주도'), send)
    expect(clipDraftState('clip')).toBe('dirty')
    await vi.advanceTimersByTimeAsync(1000)
    expect(sent).toEqual(['제주', '제주도'])
  })

  it('flushes on demand and resolves once the queue is dry', async () => {
    const { sent, settle, send } = controllable()
    queueClipDraft('clip', draft('제주'), send)
    let settled = false
    void flushClipDraft('clip').then(() => (settled = true))
    // Immediately, without waiting out the debounce — the approval cannot wait a beat.
    await vi.advanceTimersByTimeAsync(0)
    expect(sent).toEqual(['제주'])
    expect(settled).toBe(false)
    settle[0].resolve()
    await vi.advanceTimersByTimeAsync(0)
    expect(settled).toBe(true)
  })

  it('rejects a flush the server refused, so an approval cannot proceed on unsaved settings', async () => {
    const { settle, send } = controllable()
    queueClipDraft('clip', draft('제주'), send)
    const flush = flushClipDraft('clip')
    const caught = flush.catch(() => 'refused')
    await vi.advanceTimersByTimeAsync(0)
    settle[0].reject(new ConnectError('busy', Code.FailedPrecondition))
    await expect(caught).resolves.toBe('refused')
  })

  it('resolves a flush that has nothing to send', async () => {
    await expect(flushClipDraft('never-typed-in')).resolves.toBeUndefined()
  })

  it('hands a pending draft to the next mount of the form', async () => {
    const { send } = controllable()
    queueClipDraft('clip', draft('제주'), send)
    expect(peekPendingClipDraft('clip')?.title).toBe('제주')
  })

  it('drops a deleted project queue and everything it was going to send', async () => {
    const { sent, send } = controllable()
    queueClipDraft('clip', draft('제주'), send)
    discardClipDraftQueues()
    await vi.advanceTimersByTimeAsync(60_000)
    expect(sent).toEqual([])
    expect(clipDraftState('clip')).toBe('idle')
  })

  it('tells its subscribers, including one that subscribed before the first keystroke', async () => {
    const seen: string[] = []
    const stop = subscribeClipDraft('clip', () => seen.push(clipDraftState('clip')))
    const { settle, send } = controllable()
    queueClipDraft('clip', draft('제주'), send)
    await vi.advanceTimersByTimeAsync(1000)
    settle[0].resolve()
    await vi.advanceTimersByTimeAsync(0)
    expect(seen).toEqual(['dirty', 'saving', 'saved'])
    stop()
    queueClipDraft('clip', draft('제주도'), send)
    expect(seen).toHaveLength(3)
  })
})
