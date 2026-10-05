import { describe, expect, it, vi } from 'vitest'
import type { BrowserVideoTrack } from '@/entities/clip-preview'
import type { ClipProject } from '@/entities/clip-project'
import { clipTimelineFixture } from '@/test/clip-editing'
import {
  BrowserRenderSamplingError,
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
  const captionFrames = vi.fn()
  const track = { chunks: [] } as unknown as BrowserVideoTrack
  const project = { id: 'project' } as ClipProject
  const operations: BrowserRenderOperations = {
    admit: vi.fn(async () => ({ renderId: 'render', jobId: '' })),
    sampled: vi.fn(async () => undefined),
    cancel: vi.fn(async () => true),
    refresh: vi.fn(async () => project),
    prepare: vi.fn(async () => ({ assets: [], width: 1080, height: 1080, captionFrames, dispose })),
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
  const progress = vi.fn()
  const run = () => runBrowserRender(input, operations, controller.signal, progress)
  return { input, operations, controller, run, project, dispose, cancelVideo, progress }
}
describe('browser run cleanup and cancellation races', () => {
  it('cancels a late admission after navigation, before touching originals or workers', async () => {
    const f = fixture(),
      admission = deferred<{ renderId: string; jobId: string }>()
    vi.mocked(f.operations.admit).mockReturnValue(admission.promise)
    const result = f.run()
    f.controller.abort()
    admission.resolve({ renderId: 'late', jobId: 'sampling' })
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

// CLIP-192: a browser render is drawn on the grounds the server samples for it, so the page
// waits on that job as the render's own first phase and asks for its assets by name.
describe('browser run waits on its sampling job', () => {
  it('waits for the sampling job, then asks for the assets of this render', async () => {
    const f = fixture()
    vi.mocked(f.operations.admit).mockResolvedValue({ renderId: 'render', jobId: 'sampling' })
    const order: string[] = []
    vi.mocked(f.operations.sampled).mockImplementation(async () => void order.push('sampled'))
    vi.mocked(f.operations.prepare).mockImplementation(async (_input, renderId) => {
      order.push(`prepare ${renderId}`)
      return { assets: [], width: 1080, height: 1080, captionFrames: vi.fn(), dispose: f.dispose }
    })
    await f.run()
    expect(f.operations.sampled).toHaveBeenCalledExactlyOnceWith(
      'sampling',
      expect.any(AbortSignal),
    )
    expect(order).toEqual(['sampled', 'prepare render'])
    expect(f.progress).toHaveBeenCalledWith({ stage: 'sampling', percent: 0 })
  })
  it('asks for the assets straight away when no job samples the render', async () => {
    const f = fixture()
    await f.run()
    expect(f.operations.sampled).not.toHaveBeenCalled()
    expect(f.operations.prepare).toHaveBeenCalledWith(
      expect.anything(),
      'render',
      expect.any(AbortSignal),
    )
  })
  it('refuses the render when its sampling job did not finish, and withdraws it', async () => {
    const f = fixture()
    vi.mocked(f.operations.admit).mockResolvedValue({ renderId: 'render', jobId: 'sampling' })
    const failure = { reason: 'CLIP_PROCESSING_FAILED', params: {} } as const
    vi.mocked(f.operations.sampled).mockRejectedValue(new BrowserRenderSamplingError(failure))
    await expect(f.run()).rejects.toBeInstanceOf(BrowserRenderSamplingError)
    expect(f.operations.prepare).not.toHaveBeenCalled()
    expect(f.operations.video).not.toHaveBeenCalled()
    expect(f.operations.cancel).toHaveBeenCalledExactlyOnceWith('render')
  })
  it('cancels the render when the owner stops it during sampling', async () => {
    const f = fixture()
    vi.mocked(f.operations.admit).mockResolvedValue({ renderId: 'render', jobId: 'sampling' })
    vi.mocked(f.operations.sampled).mockImplementation(
      (_job, signal) =>
        new Promise<void>((_resolve, reject) =>
          signal.addEventListener('abort', () => reject(signal.reason), { once: true }),
        ),
    )
    const result = f.run()
    await vi.waitFor(() => expect(f.operations.sampled).toHaveBeenCalled())
    f.controller.abort()
    await expect(result).rejects.toMatchObject({ name: 'AbortError' })
    expect(f.operations.cancel).toHaveBeenCalledExactlyOnceWith('render')
    expect(f.operations.prepare).not.toHaveBeenCalled()
  })
})

it('refuses unresolved requested speech before browser admission and video work', async () => {
  const f = fixture()
  f.input.plan.narration = {
    enabled: true,
    confirmedVoiceId: 'voice',
    bindingDigest: 'binding',
    volumePermille: 1000,
    segments: [],
  }
  await expect(f.run()).rejects.toThrow('CLIP_BROWSER_AUDIO_SPEECH')
  expect(f.operations.admit).not.toHaveBeenCalled()
  expect(f.operations.video).not.toHaveBeenCalled()
  expect(f.operations.audio).not.toHaveBeenCalled()
})
it('does not start video or store a narrated result when its audio path refuses', async () => {
  const f = fixture()
  f.input.plan.narration = {
    enabled: true,
    confirmedVoiceId: 'voice',
    bindingDigest: 'binding',
    volumePermille: 1000,
    segments: [
      {
        id: 'spoken-1',
        text: 'sentence',
        textRevision: 1,
        inputHash: 'input',
        startMs: 1000,
        endMs: 2000,
        speech: {
          assetId: 'asset',
          voiceId: 'voice',
          bindingDigest: 'binding',
          inputHash: 'input',
          settingsHash: 'settings',
          audioHash: 'audio',
          profileId: 'profile',
          profileRevision: 1,
          samples: 44100,
          sampleRate: 44100,
          channels: 2,
          timing: [],
        },
      },
    ],
  }
  vi.mocked(f.operations.audio).mockRejectedValue(new Error('native decoder unsupported'))
  await expect(f.run()).rejects.toThrow('native decoder unsupported')
  expect(f.operations.video).not.toHaveBeenCalled()
  expect(f.operations.store).not.toHaveBeenCalled()
  expect(f.operations.cancel).not.toHaveBeenCalled()
})
