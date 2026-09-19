import { beforeEach, expect, it, vi } from 'vitest'
import { clipTimelineFixture } from '@/test/clip-editing'
import { renderBrowserVideo } from './render-video'

const { worker, sources } = vi.hoisted(() => ({
  worker: {
    postMessage: vi.fn(),
    terminate: vi.fn(),
    onmessage: vi.fn<(event: MessageEvent) => void>(),
    onerror: () => {},
    onmessageerror: () => {},
  },
  sources: { frame: vi.fn(), dispose: vi.fn() },
}))
vi.mock('@/entities/clip-preview', () => ({ createClipVideoWorker: () => worker }))
vi.mock('../lib/source-frames', () => ({ createBrowserSourceFrames: () => sources }))
beforeEach(() => vi.clearAllMocks())
const input = () => ({ plan: clipTimelineFixture().plan, ratio: 'vertical' as const, assets: [] })
const message = (data: unknown) => worker.onmessage({ data } as MessageEvent)

it('ends progress and releases a snapshot that arrives after cancellation', async () => {
  let finish!: (bitmap: ImageBitmap) => void
  sources.frame.mockImplementation(
    () =>
      new Promise<ImageBitmap>((resolve) => {
        finish = resolve
      }),
  )
  const handle = renderBrowserVideo(input(), [], vi.fn())
  const result = expect(handle.result).rejects.toMatchObject({ name: 'AbortError' })
  const progress = handle.progress[Symbol.asyncIterator]().next()
  message({ type: 'source', requestId: 1, fingerprint: 'fp', timeMs: 0 })
  handle.cancel()
  const bitmap = { close: vi.fn() } as unknown as ImageBitmap
  finish(bitmap)
  await result
  expect(await progress).toEqual({ done: true, value: undefined })
  expect(bitmap.close).toHaveBeenCalledOnce()
  expect(worker.terminate).toHaveBeenCalledOnce()
  expect(sources.dispose).toHaveBeenCalledOnce()
  expect(worker.postMessage).toHaveBeenCalledTimes(1)
})

it('rejects a failed source read and cleans up the waiting worker', async () => {
  sources.frame.mockRejectedValue(new Error('expired original'))
  const handle = renderBrowserVideo(input(), [], vi.fn())
  const result = expect(handle.result).rejects.toThrow('expired original')
  message({ type: 'source', requestId: 1, fingerprint: 'fp', timeMs: 0 })
  await result
  expect(worker.terminate).toHaveBeenCalledOnce()
  expect(sources.dispose).toHaveBeenCalledOnce()
})

it('keeps the final completed-frame count and returns the worker track unchanged', async () => {
  const handle = renderBrowserVideo(input(), [], vi.fn())
  const track = { frameCount: 30, durationUs: 1_000_000, chunks: [] }
  message({ type: 'progress', progress: { completedFrames: 30, totalFrames: 30 } })
  message({ type: 'done', track })
  expect(await handle.result).toBe(track)
  const progress = []
  for await (const value of handle.progress) progress.push(value)
  expect(progress).toEqual([{ completedFrames: 30, totalFrames: 30 }])
  handle.cancel()
  expect(worker.terminate).toHaveBeenCalledOnce()
})
