import { afterEach, expect, it, vi } from 'vitest'
import { createAudioRangeReader } from './range-reader'
import type { AudioRangeLimits } from './range-audio'

const limits: AudioRangeLimits = {
  maxFileBytes: 10000,
  maxReadBytes: 1000,
  maxReadTotalBytes: 10000,
  maxCacheBytes: 1000,
  timeoutMs: 1000,
  maxPcmBytes: 10000,
  maxChannels: 2,
  maxSampleRate: 48000,
  maxSampleFrames: 2048,
  decoderPrerollMs: 125,
  decoderTailMs: 125,
  decoderReserveSamples: 2,
  operationTimeoutMs: 1000,
  cleanupTimeoutMs: 50,
}
class FakeWorker {
  static current: FakeWorker
  onmessage?: (event: MessageEvent) => void
  onerror?: (event: ErrorEvent) => void
  onmessageerror?: () => void
  postMessage = vi.fn()
  terminate = vi.fn()
  constructor() {
    FakeWorker.current = this
  }
  respond(data: unknown) {
    this.onmessage?.({ data } as MessageEvent)
  }
}
afterEach(() => {
  vi.unstubAllGlobals()
  vi.useRealTimers()
})
it('admits one active range and ignores old PCM after a bounded cancellation handshake', async () => {
  vi.useFakeTimers()
  vi.stubGlobal('Worker', FakeWorker)
  const controller = new AbortController(),
    reader = createAudioRangeReader(controller.signal, limits),
    worker = FakeWorker.current
  const access = { kind: 'url', url: 'https://example.com/original' } as const
  const range = { startUs: 0, endUs: 1000000, targetSampleRate: 48000 }
  const pending = reader.decode(access, range)
  await expect(reader.decode(access, range)).rejects.toThrow('CLIP_SOURCE_AUDIO_QUEUE_LIMIT')
  const stopped = expect(pending).rejects.toThrow('stopped')
  controller.abort(new Error('stopped'))
  await stopped
  worker.respond({ kind: 'result', id: 1, result: { channels: [new Float32Array(4)] } })
  expect(worker.terminate).not.toHaveBeenCalled()
  worker.respond({ kind: 'cancelled' })
  expect(worker.terminate).toHaveBeenCalledTimes(1)
  expect(vi.getTimerCount()).toBe(0)
})
it('terminates a nonresponding worker after the operation and cleanup deadlines', async () => {
  vi.useFakeTimers()
  vi.stubGlobal('Worker', FakeWorker)
  const reader = createAudioRangeReader(new AbortController().signal, limits),
    worker = FakeWorker.current
  const pending = reader.decode(
    { kind: 'blob', blob: new Blob() },
    { startUs: 0, endUs: 1000000, targetSampleRate: 48000 },
  )
  const timedOut = expect(pending).rejects.toThrow('CLIP_SOURCE_AUDIO_TIMEOUT')
  await vi.advanceTimersByTimeAsync(1000)
  await timedOut
  await vi.advanceTimersByTimeAsync(50)
  expect(worker.terminate).toHaveBeenCalledOnce()
  expect(vi.getTimerCount()).toBe(0)
})
