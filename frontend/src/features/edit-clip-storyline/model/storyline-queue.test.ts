import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { Code, ConnectError } from '@connectrpc/connect'
import { serialClipWrite, type ClipStorylineParagraph } from '@/entities/clip-project'
import { connectAppError } from '@/test/app-error'
import {
  clipStorylineFailure,
  clipStorylineState,
  discardClipStorylineQueues,
  flushClipStoryline,
  peekClipStoryline,
  queueClipStoryline,
} from './storyline-queue'

beforeEach(() => vi.useFakeTimers())
afterEach(() => {
  discardClipStorylineQueues()
  vi.useRealTimers()
})

const story = (text: string): ClipStorylineParagraph[] => [{ text, observationIds: ['a/0'] }]

/** A send whose promises are settled by the test, so a flight can be held open deliberately. */
function controllable() {
  const sent: string[] = []
  const settle: { resolve: () => void; reject: (error: unknown) => void }[] = []
  const send = (paragraphs: ClipStorylineParagraph[]) => {
    sent.push(paragraphs[0]!.text)
    return new Promise<void>((resolve, reject) => settle.push({ resolve, reject }))
  }
  return { sent, settle, send }
}

it('saves once a beat after the last edit', async () => {
  const { sent, settle, send } = controllable()
  queueClipStoryline('clip', story('음식'), send)
  queueClipStoryline('clip', story('음식을 가까이'), send)
  await vi.advanceTimersByTimeAsync(600)
  expect(sent).toEqual(['음식을 가까이'])
  settle[0]!.resolve()
  await vi.advanceTimersByTimeAsync(0)
  expect(clipStorylineState('clip')).toBe('saved')
  expect(peekClipStoryline('clip')).toBeUndefined()
})

it('retries a transient failure and keeps the paragraphs owed meanwhile', async () => {
  const { sent, settle, send } = controllable()
  queueClipStoryline('clip', story('음식을 가까이'), send)
  await vi.advanceTimersByTimeAsync(600)
  settle[0]!.reject(new ConnectError('offline', Code.Unavailable))
  await vi.advanceTimersByTimeAsync(0)
  expect(clipStorylineState('clip')).toBe('error')
  expect(peekClipStoryline('clip')?.[0]?.text).toBe('음식을 가까이')
  expect(clipStorylineFailure('clip')).toBeDefined()
  await vi.advanceTimersByTimeAsync(1000)
  expect(sent).toEqual(['음식을 가까이', '음식을 가까이'])
  settle[1]!.resolve()
  await vi.advanceTimersByTimeAsync(0)
  expect(clipStorylineState('clip')).toBe('saved')
  expect(clipStorylineFailure('clip')).toBeUndefined()
})

it('stops on a refusal, keeping the paragraphs and the reason, until the next edit', async () => {
  const { sent, settle, send } = controllable()
  queueClipStoryline('clip', story('음식을 가까이'), send)
  await vi.advanceTimersByTimeAsync(600)
  settle[0]!.reject(connectAppError('CLIP_STORYLINE_INVALID', Code.InvalidArgument))
  await vi.advanceTimersByTimeAsync(60_000)
  expect(sent).toHaveLength(1)
  expect(clipStorylineState('clip')).toBe('refused')
  expect(clipStorylineFailure('clip')?.reason).toBe('CLIP_STORYLINE_INVALID')
  expect(peekClipStoryline('clip')?.[0]?.text).toBe('음식을 가까이')
})

it('sends each project’s paragraphs through that project’s sender', async () => {
  const a = controllable()
  const b = controllable()
  queueClipStoryline('a', story('A'), a.send)
  queueClipStoryline('b', story('B'), b.send)
  await vi.advanceTimersByTimeAsync(600)
  expect(a.sent).toEqual(['A'])
  expect(b.sent).toEqual(['B'])
})

it('waits its turn in the project’s write lane, and a flush waits for it to land', async () => {
  let releaseEarlier!: () => void
  const order: string[] = []
  void serialClipWrite('clip', async () => {
    await new Promise<void>((resolve) => (releaseEarlier = resolve))
    order.push('earlier write')
  })
  queueClipStoryline('clip', story('음식'), async () => {
    order.push('storyline')
  })
  let flushed = false
  void flushClipStoryline('clip', true).then(() => (flushed = true))
  await vi.advanceTimersByTimeAsync(0)
  expect(order).toEqual([])
  expect(flushed).toBe(false)
  releaseEarlier()
  await vi.advanceTimersByTimeAsync(0)
  expect(order).toEqual(['earlier write', 'storyline'])
  expect(flushed).toBe(true)
})
