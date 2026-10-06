import { afterEach, expect, it, vi } from 'vitest'
import { createAudioProcessor } from './processing'

const limits = {
  maxPcmBytes: 1024,
  maxEncodedBytes: 1024,
  maxEncodedPackets: 8,
  maxPacketBytes: 128,
  maxPrimingFrames: 2048,
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
it('refuses overlapping or excessive DSP inputs before transferring their owned PCM', async () => {
  vi.useFakeTimers()
  vi.stubGlobal('Worker', FakeWorker)
  const processor = createAudioProcessor(new AbortController().signal, limits),
    worker = FakeWorker.current
  const channels = [new Float32Array(64), new Float32Array(64)]
  expect(() => processor.stretch(channels, 48000, 0.5, 512)).toThrow('AUDIO_PCM_MEMORY_LIMIT')
  expect(worker.postMessage).not.toHaveBeenCalled()
  const pending = processor.stretch(channels, 48000, 1, 64)
  expect(() => processor.normalize(channels, -16, -1.5)).toThrow('AUDIO_WORKER_QUEUE_LIMIT')
  worker.respond({ kind: 'result', id: 1, result: channels })
  expect(await pending).toBe(channels)
  processor.close()
  worker.respond({ kind: 'cancelled' })
  expect(vi.getTimerCount()).toBe(0)
})
it('fences late results and gives active codecs a finite cleanup window on abort', async () => {
  vi.useFakeTimers()
  vi.stubGlobal('Worker', FakeWorker)
  const controller = new AbortController(),
    processor = createAudioProcessor(controller.signal, limits),
    worker = FakeWorker.current
  const pending = processor.normalize([new Float32Array(64), new Float32Array(64)], -16, -1.5)
  const stopped = expect(pending).rejects.toThrow('cancelled')
  controller.abort(new Error('cancelled'))
  await stopped
  worker.respond({ kind: 'result', id: 1, result: { channels: [] } })
  await vi.advanceTimersByTimeAsync(50)
  expect(worker.terminate).toHaveBeenCalledOnce()
  expect(vi.getTimerCount()).toBe(0)
})
