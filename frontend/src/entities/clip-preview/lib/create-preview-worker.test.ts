import { afterEach, expect, it, vi } from 'vitest'
import { BrowserPreviewWorker } from './create-preview-worker'
import type { BrowserCompositionSnapshot } from '../model/browser-composition'
import type {
  BrowserPreviewWorkerInput,
  BrowserPreviewWorkerOutput,
} from '../model/preview-worker-protocol'

function setup() {
  const port = {
    postMessage: vi.fn(),
    terminate: vi.fn(),
    onmessage: vi.fn<(event: MessageEvent<BrowserPreviewWorkerOutput>) => void>(),
  }
  const worker = new BrowserPreviewWorker(vi.fn(), () => port as unknown as Worker)
  const send = (data: BrowserPreviewWorkerOutput) =>
    port.onmessage({ data } as MessageEvent<BrowserPreviewWorkerOutput>)
  const last = () => port.postMessage.mock.lastCall![0] as BrowserPreviewWorkerInput
  return { worker, port, send, last }
}
async function initialized() {
  const value = setup()
  const pending = value.worker.initialize(
    { snapshotFingerprint: 'current' } as BrowserCompositionSnapshot,
    new AbortController().signal,
  )
  value.send({ type: 'initialized', id: (value.last() as { id: number }).id })
  await pending
  return value
}
afterEach(() => vi.useRealTimers())
it('allows bounded whole-plan preparation beyond a single frame deadline, then retains the frame deadline', async () => {
  vi.useFakeTimers()
  const { worker, send, last } = await initialized()
  const progress = vi.fn()
  const pending = worker.prepare(new AbortController().signal, progress)
  const request = last() as Extract<BrowserPreviewWorkerInput, { type: 'prepare' }>
  await vi.advanceTimersByTimeAsync(45000)
  send({ type: 'preparing', id: request.id, epoch: request.epoch, done: 3, total: 6 })
  expect(progress).toHaveBeenCalledWith(3, 6)
  send({ type: 'prepared', id: request.id, epoch: request.epoch })
  await pending
  const frame = worker.render({ frame: 0, flow: false }, new AbortController().signal)
  const refused = expect(frame).rejects.toThrow('CLIP_PREVIEW_TIMEOUT')
  await vi.advanceTimersByTimeAsync(30000)
  await refused
  worker.dispose()
})
it('bounds preparation itself and refuses progress after cancellation', async () => {
  vi.useFakeTimers()
  const { worker, send, last } = await initialized()
  const progress = vi.fn()
  const pending = worker.prepare(new AbortController().signal, progress)
  const request = last() as Extract<BrowserPreviewWorkerInput, { type: 'prepare' }>
  const refused = expect(pending).rejects.toThrow('CLIP_PREVIEW_TIMEOUT')
  await vi.advanceTimersByTimeAsync(120000)
  await refused
  send({ type: 'preparing', id: request.id, epoch: request.epoch, done: 6, total: 6 })
  expect(progress).not.toHaveBeenCalled()
  worker.dispose()
})
it('fences superseded preparation and never publishes late progress or completion', async () => {
  const { worker, send, last } = await initialized()
  const progress = vi.fn()
  const pending = worker.prepare(new AbortController().signal, progress)
  const request = last() as Extract<BrowserPreviewWorkerInput, { type: 'prepare' }>
  const refused = expect(pending).rejects.toThrow('CLIP_SNAPSHOT_SUPERSEDED')
  worker.cancel()
  await refused
  send({ type: 'preparing', id: request.id, epoch: request.epoch, done: 1, total: 3 })
  send({ type: 'prepared', id: request.id, epoch: request.epoch })
  expect(progress).not.toHaveBeenCalled()
  worker.dispose()
})
