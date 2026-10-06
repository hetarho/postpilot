import { beforeEach, expect, it, vi } from 'vitest'
import { clipTimelineFixture } from '@/test/clip-editing'
import type { CaptionFrameLoader, BrowserCompositionSnapshot } from '@/entities/clip-preview'
import { renderBrowserVideo } from './render-video'

const { worker, sources, sheets } = vi.hoisted(() => ({
  worker: {
    postMessage: vi.fn(),
    terminate: vi.fn(),
    onmessage: vi.fn<(event: MessageEvent) => void>(),
    onerror: () => {},
    onmessageerror: () => {},
  },
  sources: { frame: vi.fn(), dispose: vi.fn() },
  sheets: { dispose: vi.fn() },
}))
vi.mock('@/entities/clip-preview', () => ({
  CLIP_VIDEO_DECODING: { workerCleanupMs: 1000 },
  createClipVideoWorker: () => worker,
  CaptionSheets: class {
    constructor(private load: (id: string, frame: number, signal: AbortSignal) => unknown) {}
    cell = (id: string, frame: number, signal: AbortSignal) => this.load(id, frame, signal)
    dispose = sheets.dispose
  },
}))
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

it('answers a caption frame from the sheets and releases them with the render', async () => {
  const bitmap = { close: vi.fn() } as unknown as ImageBitmap
  // `CaptionSheets` is mocked to forward to the loader, so this stands in for the
  // cell it would cut out of a sheet.
  const load = vi.fn(async () => ({ bitmap, x: 90, y: 640, width: 900, height: 250 }))
  const handle = renderBrowserVideo(
    input(),
    [],
    vi.fn(),
    undefined,
    undefined,
    load as unknown as CaptionFrameLoader,
  )
  message({ type: 'frames', requestId: 7, instanceId: 'caption', frame: 42 })
  await vi.waitFor(() => expect(worker.postMessage).toHaveBeenCalled())
  expect(load).toHaveBeenCalledWith('caption', 42, expect.anything())
  expect(worker.postMessage).toHaveBeenCalledWith(
    { type: 'frames', requestId: 7, bitmap, x: 90, y: 640, width: 900, height: 250 },
    [bitmap],
  )
  handle.cancel()
  await expect(handle.result).rejects.toMatchObject({ name: 'AbortError' })
  expect(sheets.dispose).toHaveBeenCalledOnce()
})

it('refuses the render when no frame source was handed over', async () => {
  const handle = renderBrowserVideo(input(), [], vi.fn())
  const result = expect(handle.result).rejects.toThrow('CLIP_CAPTION_FRAMES_UNAVAILABLE')
  message({ type: 'frames', requestId: 1, instanceId: 'caption', frame: 0 })
  await result
  expect(worker.terminate).toHaveBeenCalledOnce()
})
it('answers owner-bound source access once per original without any per-frame DOM requests', async () => {
  const frozen = { ...input(), snapshot: {} as BrowserCompositionSnapshot }
  const file = new File(['original'], 'source.mp4')
  const resolve = vi.fn(async () => 'fresh-private-access')
  const handle = renderBrowserVideo(
    frozen,
    [{ fingerprint: 'local', url: 'blob:file', file }],
    resolve,
  )
  message({ type: 'sourceAccess', requestId: 1, sourceId: 'local', fingerprint: 'local' })
  message({ type: 'sourceAccess', requestId: 2, sourceId: 'remote', fingerprint: 'remote' })
  message({ type: 'sourceAccess', requestId: 3, sourceId: 'remote', fingerprint: 'remote' })
  await vi.waitFor(() => expect(worker.postMessage).toHaveBeenCalledTimes(4))
  expect(resolve).toHaveBeenCalledOnce()
  expect(sources.frame).not.toHaveBeenCalled()
  expect(worker.postMessage).toHaveBeenCalledWith(
    { type: 'sourceAccess', requestId: 1, access: { kind: 'blob', blob: file } },
    [],
  )
  handle.cancel()
  await expect(handle.result).rejects.toMatchObject({ name: 'AbortError' })
  expect(worker.postMessage).toHaveBeenLastCalledWith({ type: 'cancel' }, [])
  message({ type: 'cancelled' })
  expect(worker.terminate).toHaveBeenCalledOnce()
})
it('cannot publish a resolved access descriptor into a cancelled frozen run', async () => {
  let finish!: (url: string) => void
  const handle = renderBrowserVideo(
    { ...input(), snapshot: {} as BrowserCompositionSnapshot },
    [],
    () =>
      new Promise<string>((resolve) => {
        finish = resolve
      }),
  )
  message({ type: 'sourceAccess', requestId: 1, sourceId: 'remote', fingerprint: 'remote' })
  handle.cancel()
  finish('late-private-access')
  await expect(handle.result).rejects.toMatchObject({ name: 'AbortError' })
  await Promise.resolve()
  expect(worker.postMessage).toHaveBeenCalledTimes(2)
  message({ type: 'cancelled' })
})
