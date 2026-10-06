import { expect, it, vi } from 'vitest'
import { waitAudioCodecCapacity, drainAudioCodec } from './codec-queue'

it('waits for capacity without flushing a healthy encoder and stops promptly on abort', async () => {
  const codec = Object.assign(new EventTarget(), { encodeQueueSize: 4, flush: vi.fn() })
  const controller = new AbortController()
  let finished = false
  const pending = waitAudioCodecCapacity(codec, 4, controller.signal, 1000).then(() => {
    finished = true
  })
  await Promise.resolve()
  expect(finished).toBe(false)
  codec.encodeQueueSize = 3
  codec.dispatchEvent(new Event('dequeue'))
  await pending
  expect(codec.flush).not.toHaveBeenCalled()
  codec.encodeQueueSize = 4
  const blocked = waitAudioCodecCapacity(codec, 4, controller.signal, 1000)
  controller.abort(new Error('decoder failed'))
  await expect(blocked).rejects.toThrow('decoder failed')
})
it('bounds a stalled final drain and releases its listeners after cancellation', async () => {
  vi.useFakeTimers()
  try {
    const controller = new AbortController()
    const codec = { flush: vi.fn(() => new Promise<void>(() => {})) }
    const pending = drainAudioCodec(codec, controller.signal, 50)
    const refused = expect(pending).rejects.toThrow('AUDIO_CODEC_TIMEOUT')
    await vi.advanceTimersByTimeAsync(50)
    await refused
    expect(vi.getTimerCount()).toBe(0)
  } finally {
    vi.useRealTimers()
  }
})
