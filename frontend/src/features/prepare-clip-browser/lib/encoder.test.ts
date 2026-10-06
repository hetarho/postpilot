import { afterEach, describe, expect, it, vi } from 'vitest'
import { createAnalysisEncoder } from './encoder'

class OwnedWorker {
  static instances: OwnedWorker[] = []
  onmessage?: (event: MessageEvent) => void
  onerror?: (event: ErrorEvent) => void
  onmessageerror?: () => void
  postMessage = vi.fn()
  terminate = vi.fn()
  constructor() {
    OwnedWorker.instances.push(this)
  }
}
const source = {
  sourceId: 's',
  fingerprint: 'a'.repeat(64),
  access: { kind: 'blob' as const, blob: new Blob(['source']) },
}
afterEach(() => {
  vi.unstubAllGlobals()
  vi.useRealTimers()
  OwnedWorker.instances = []
})
describe('bounded analysis worker ownership', () => {
  it('permits one operation and releases an owned worker after cancellation acknowledgment', async () => {
    vi.stubGlobal('Worker', OwnedWorker)
    const controller = new AbortController(),
      encoder = createAnalysisEncoder(controller.signal)
    const worker = OwnedWorker.instances[0],
      pending = encoder.measure(source)
    await expect(encoder.measure(source)).rejects.toThrow('CLIP_ANALYSIS_QUEUE_LIMIT')
    controller.abort()
    await expect(pending).rejects.toThrow()
    expect(worker.postMessage).toHaveBeenLastCalledWith({ kind: 'cancel' })
    worker.onmessage?.({ data: { kind: 'cancelled' } } as MessageEvent)
    expect(worker.terminate).toHaveBeenCalledTimes(1)
  })
  it('ignores stale progress/results and forcibly terminates within the cleanup bound', async () => {
    vi.useFakeTimers()
    vi.stubGlobal('Worker', OwnedWorker)
    const controller = new AbortController(),
      progress = vi.fn(),
      encoder = createAnalysisEncoder(controller.signal, progress)
    const worker = OwnedWorker.instances[0],
      pending = encoder.measure(source)
    worker.onmessage?.({ data: { kind: 'progress', id: 999, fraction: 1 } } as MessageEvent)
    expect(progress).not.toHaveBeenCalled()
    controller.abort()
    await expect(pending).rejects.toThrow()
    worker.onmessage?.({ data: { kind: 'result', id: 1, result: 'late bytes' } } as MessageEvent)
    await vi.advanceTimersByTimeAsync(1000)
    expect(worker.terminate).toHaveBeenCalledTimes(1)
    await expect(encoder.measure(source)).rejects.toThrow()
  })
  it('reports missing worker capability by name', () => {
    vi.stubGlobal('Worker', undefined)
    expect(() => createAnalysisEncoder(new AbortController().signal)).toThrow(
      'CLIP_ANALYSIS_ENCODER_UNSUPPORTED',
    )
  })
})
