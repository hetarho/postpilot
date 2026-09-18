import { describe, expect, it, vi } from 'vitest'
import type { ClipProject, BrowserVideoTrack } from '@/entities/clip-project'
import { clipTimelineFixture } from '@/test/clip-editing'
import {
  runBrowserRender,
  type BrowserRenderOperations,
  type BrowserRenderInput,
} from './run-render'

function deferred<T>() {
  let resolve!: (value: T) => void, reject!: (error: unknown) => void
  const promise = new Promise<T>((yes, no) => {
    resolve = yes
    reject = no
  })
  return { promise, resolve, reject }
}
function fixture() {
  const input: BrowserRenderInput = {
    projectId: 'project',
    revision: 1,
    batchId: 'batch',
    plan: clipTimelineFixture().plan,
    ratio: 'square',
    localSources: [],
    resolvePlayback: vi.fn(),
  }
  const dispose = vi.fn(),
    cancelVideo = vi.fn()
  const track = { chunks: [] } as unknown as BrowserVideoTrack
  const project = { id: 'project' } as ClipProject
  const operations: BrowserRenderOperations = {
    admit: vi.fn(async () => 'render'),
    cancel: vi.fn(async () => true),
    refresh: vi.fn(async () => project),
    prepare: vi.fn(async () => ({ assets: [], width: 1080, height: 1080, dispose })),
    video: vi.fn(() => ({
      result: Promise.resolve(track),
      cancel: cancelVideo,
      progress: {
        async *[Symbol.asyncIterator]() {
          yield { completedFrames: 1, totalFrames: 1 }
        },
      },
    })),
    audio: vi.fn(async () => undefined),
    store: vi.fn(async () => project),
  }
  const controller = new AbortController()
  const run = () => runBrowserRender(input, operations, controller.signal, vi.fn())
  return { operations, controller, run, project, dispose, cancelVideo }
}
describe('browser run cleanup and cancellation races', () => {
  it('cancels a late admission after navigation, before touching originals or workers', async () => {
    const f = fixture(),
      admission = deferred<string>()
    vi.mocked(f.operations.admit).mockReturnValue(admission.promise)
    const result = f.run()
    f.controller.abort()
    admission.resolve('late')
    await expect(result).rejects.toMatchObject({ name: 'AbortError' })
    expect(f.operations.cancel).toHaveBeenCalledExactlyOnceWith('late')
    expect(f.operations.prepare).not.toHaveBeenCalled()
    expect(f.operations.video).not.toHaveBeenCalled()
    expect(f.operations.store).not.toHaveBeenCalled()
  })
  it('stops the sibling audio worker after video fails and disposes prepared assets', async () => {
    const f = fixture()
    let signal: AbortSignal | undefined
    vi.mocked(f.operations.video).mockReturnValue({
      result: Promise.reject(new Error('encoder failed')),
      cancel: f.cancelVideo,
      progress: {
        async *[Symbol.asyncIterator]() {
          yield { completedFrames: 0, totalFrames: 1 }
        },
      },
    })
    vi.mocked(f.operations.audio).mockImplementation(async (_plan, _ratio, _originals, active) => {
      signal = active
      await new Promise<void>((_resolve, reject) =>
        active.addEventListener('abort', () => reject(active.reason), { once: true }),
      )
      return undefined
    })
    await expect(f.run()).rejects.toThrow('encoder failed')
    expect(signal?.aborted).toBe(true)
    expect(f.dispose).toHaveBeenCalledOnce()
    expect(f.operations.cancel).toHaveBeenCalledExactlyOnceWith('render')
    expect(f.operations.store).not.toHaveBeenCalled()
  })
  it('returns the stored projection when completion won before cancellation', async () => {
    const f = fixture(),
      storing = deferred<void>()
    vi.mocked(f.operations.cancel).mockResolvedValue(false)
    vi.mocked(f.operations.store).mockImplementation(
      async (_id, _video, _audio, _input, signal) => {
        storing.resolve()
        return new Promise((_resolve, reject) =>
          signal.addEventListener('abort', () => reject(signal.reason), { once: true }),
        )
      },
    )
    const result = f.run()
    await storing.promise
    f.controller.abort()
    expect(await result).toBe(f.project)
    expect(f.operations.refresh).toHaveBeenCalledExactlyOnceWith('project')
    expect(f.operations.cancel).toHaveBeenCalledExactlyOnceWith('render')
    expect(f.dispose).toHaveBeenCalledOnce()
  })
})
