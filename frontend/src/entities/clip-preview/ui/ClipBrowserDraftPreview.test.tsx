import { StrictMode } from 'react'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { clipTimelineFixture } from '@/test/clip-editing'
import type { BrowserCompositionSnapshot } from '../model/browser-composition'
import type { ClipLocalCompositionRuntime } from '../model/local-preview'
import { ClipBrowserDraftPreview } from './ClipBrowserDraftPreview'

const failure = vi.hoisted(() => ({
  code: '',
  active: false,
  holdPrepare: false,
  holdFrame: false,
  holdPlay: false,
  playbacks: [] as Array<() => void>,
  events: [] as string[],
  preparations: [] as Array<() => void>,
  frames: [] as Array<() => void>,
  transports: [] as Array<{
    running: boolean
    position: number
    play: ReturnType<typeof vi.fn>
    dispose: ReturnType<typeof vi.fn>
  }>,
}))
vi.mock('../model/composition-playback', () => ({
  BrowserCompositionPlayback: class {
    running = false
    position = 0
    epoch = 0
    constructor() {
      failure.transports.push(this)
    }
    get timeMs() {
      return this.position
    }
    setMuted = vi.fn()
    pause = vi.fn(() => {
      this.running = false
      this.epoch++
      failure.events.push('pause')
    })
    play = vi.fn((position: number) => {
      const own = this.epoch
      this.position = position
      const finish = () => {
        if (own !== this.epoch) return false
        this.running = true
        failure.events.push('play')
        return true
      }
      return failure.holdPlay
        ? new Promise<boolean>((resolve) => failure.playbacks.push(() => resolve(finish())))
        : Promise.resolve(finish())
    })
    dispose = vi.fn(() => {
      this.running = false
    })
  },
}))
vi.mock('../lib/create-preview-worker', () => ({
  BrowserPreviewWorker: class {
    snapshot?: BrowserCompositionSnapshot
    initialize = async (snapshot: BrowserCompositionSnapshot) => {
      this.snapshot = snapshot
      failure.events.push(`initialize:${snapshot.snapshotFingerprint}`)
      if (failure.code) throw new Error(failure.code)
    }
    prepare = (signal: AbortSignal, progress: (done: number, total: number) => void) => {
      progress(1, 3)
      if (failure.active) {
        if (!failure.holdPrepare) return Promise.resolve()
        return new Promise<void>((resolve) => failure.preparations.push(resolve))
      }
      return new Promise<void>((_resolve, reject) => {
        signal.addEventListener('abort', () => reject(signal.reason), { once: true })
      })
    }
    render = (request: { frame: number; flow: boolean }) => {
      const fingerprint = this.snapshot!.snapshotFingerprint
      const result = {
        frame: request.frame,
        flow: request.flow,
        fingerprint,
        bitmap: { fingerprint, close: vi.fn(() => failure.events.push(`close:${fingerprint}`)) },
        displayed: [],
        resources: { liveFrames: 0 },
        backgroundMeasured: true,
      }
      if (failure.holdFrame)
        return new Promise((resolve) => failure.frames.push(() => resolve(result)))
      return Promise.resolve(result)
    }
    cancel = vi.fn()
    dispose = vi.fn()
  },
}))
afterEach(() => {
  cleanup()
  failure.active = false
  failure.holdPrepare = false
  failure.holdFrame = false
  failure.holdPlay = false
  failure.playbacks.length = 0
  failure.events.length = 0
  failure.preparations.length = 0
  failure.frames.length = 0
  failure.transports.length = 0
  failure.code = ''
  vi.restoreAllMocks()
})
it('shows scene preparation and keeps it alive through playhead changes before enabling playback', async () => {
  failure.code = ''
  const plan = clipTimelineFixture().plan
  const snapshot = {
    ownerId: 'owner',
    projectId: 'project',
    versions: {},
    sources: [],
    snapshotFingerprint: 'current',
    frameCount: 594,
    plan,
  } as unknown as BrowserCompositionSnapshot
  const props = {
    ratio: 'vertical',
    plan,
    local: { snapshot, source: vi.fn(), audioSource: vi.fn() },
    preview: {
      ready: true,
      updating: false,
      assets: [],
      canvasWidth: 1080,
      canvasHeight: 1920,
      onRetry: vi.fn(),
    },
  }
  const view = render(<ClipBrowserDraftPreview {...props} timeMs={0} />)
  expect(await screen.findByText('미리보기 장면을 확인하고 있어요. 1/3')).toBeInTheDocument()
  view.rerender(<ClipBrowserDraftPreview {...props} timeMs={1000} />)
  expect(screen.getByText('미리보기 장면을 확인하고 있어요. 1/3')).toBeInTheDocument()
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})
it.each([
  ['CLIP_SOURCE_CODEC_UNSUPPORTED', '원본 형식을 지원하는 브라우저나 기기에서 열어 주세요.'],
  ['CLIP_SOURCE_MEMORY_LIMIT', '원본을 미리 보는 데 필요한 메모리가 부족해요.'],
  ['CLIP_PREVIEW_TIMEOUT', '원본을 읽는 데 시간이 오래 걸려 미리보기가 멈췄어요.'],
])(
  'identifies %s and preserves the retry action without starting an export',
  async (code, message) => {
    failure.code = code
    const plan = clipTimelineFixture().plan
    const snapshot = {
      ownerId: 'owner',
      projectId: 'project',
      versions: {},
      sources: [],
      snapshotFingerprint: 'current',
      frameCount: 594,
      plan,
    } as unknown as BrowserCompositionSnapshot
    render(
      <ClipBrowserDraftPreview
        ratio="vertical"
        plan={plan}
        local={{ snapshot, source: vi.fn(), audioSource: vi.fn() }}
        preview={{
          ready: true,
          updating: false,
          assets: [],
          canvasWidth: 1080,
          canvasHeight: 1920,
          onRetry: vi.fn(),
        }}
      />,
    )
    expect(await screen.findByText(new RegExp(message))).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '미리보기 다시 준비' })).toBeEnabled()
    expect(screen.queryByRole('button', { name: /서버.*승인/ })).not.toBeInTheDocument()
  },
)

function liveRuntime(
  revision = 1,
  marker = new AbortController().signal,
  leaseEpoch = 0,
): ClipLocalCompositionRuntime {
  const plan = clipTimelineFixture().plan
  return {
    snapshot: {
      ownerId: 'owner',
      projectId: 'project',
      projectRevision: revision,
      planRevision: revision,
      versions: {},
      sources: [],
      snapshotFingerprint: `revision-${revision}`,
      frameCount: Math.ceil((plan.durationMs * 30) / 1000),
      plan,
      // Nested provenance is part of compatibility, even though the top hash is omitted.
      ink: { fingerprint: 'nested-current' },
    } as unknown as BrowserCompositionSnapshot,
    continuity: { signal: marker, leaseEpoch, retainedUntil: Date.parse('2099-01-01T00:00:00Z') },
    source: vi.fn(async () => {
      failure.events.push('source')
      return { kind: 'url' as const, url: 'https://owned.test/current' }
    }),
    audioSource: vi.fn(),
  }
}
function liveProps(runtime: ClipLocalCompositionRuntime) {
  return {
    ratio: 'vertical',
    plan: runtime.snapshot.plan as import('@/entities/clip-plan').ClipEditPlan,
    local: runtime as ClipLocalCompositionRuntime | undefined,
    preview: {
      ready: true,
      updating: false,
      assets: [],
      canvasWidth: 1080,
      canvasHeight: 1920,
      onRetry: vi.fn(),
    },
  }
}
function enableLive() {
  failure.active = true
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({
    drawImage: (bitmap: { fingerprint: string }) =>
      failure.events.push(`draw:${bitmap.fingerprint}`),
    clearRect: vi.fn(),
  } as unknown as ReturnType<HTMLCanvasElement['getContext']>)
}
async function playLive(strict = false) {
  enableLive()
  const controller = new AbortController(),
    runtime = liveRuntime(1, controller.signal)
  const props = liveProps(runtime)
  const view = render(<ClipBrowserDraftPreview {...props} />, {
    wrapper: strict ? StrictMode : undefined,
  })
  await waitFor(() => expect(screen.getByRole('button', { name: '재생' })).toBeEnabled())
  fireEvent.click(screen.getByRole('button', { name: '재생' }))
  await waitFor(() => expect(failure.transports.some((transport) => transport.running)).toBe(true))
  const transport = failure.transports.find((candidate) => candidate.running)!
  transport.position = 3533
  return { controller, runtime, props, view, transport: failure.transports[0]! }
}

it('pauses across an absent runtime and resumes the SAME transport only after fresh access and DRAWN frame', async () => {
  const { controller, props, view, transport } = await playLive()
  failure.holdPrepare = true
  failure.holdFrame = true
  view.rerender(<ClipBrowserDraftPreview {...props} local={undefined} />)
  expect(transport.running).toBe(false)
  expect(screen.getByRole('button', { name: '일시 정지' })).toBeEnabled()
  const next = liveRuntime(2, controller.signal)
  view.rerender(<ClipBrowserDraftPreview {...props} local={next} />)
  await waitFor(() => expect(failure.preparations).toHaveLength(1))
  expect(transport.play).toHaveBeenCalledTimes(1)
  await act(async () => failure.preparations.shift()!())
  await waitFor(() => expect(failure.frames.length).toBeGreaterThan(0))
  expect(next.source).toHaveBeenCalled()
  expect(transport.play).toHaveBeenCalledTimes(1)
  await act(async () => failure.frames.splice(0).forEach((finish) => finish()))
  await waitFor(() => expect(transport.play).toHaveBeenCalledTimes(2))
  expect(transport.play).toHaveBeenLastCalledWith(3533)
  expect(failure.transports).toHaveLength(1)
  expect(failure.events.lastIndexOf('draw:revision-2')).toBeLessThan(
    failure.events.lastIndexOf('play'),
  )
  view.unmount()
  expect(transport.dispose).toHaveBeenCalledOnce()
})

it('revalidates a same-fingerprint lease runtime before continuing at unchanged revisions', async () => {
  const { controller, runtime, props, view, transport } = await playLive()
  failure.holdPrepare = true
  view.rerender(<ClipBrowserDraftPreview {...props} local={undefined} />)
  const next = { ...liveRuntime(1, controller.signal, 1), snapshot: runtime.snapshot }
  view.rerender(<ClipBrowserDraftPreview {...props} local={next} />)
  expect(transport.running).toBe(false)
  await waitFor(() => expect(failure.preparations).toHaveLength(1))
  expect(transport.play).toHaveBeenCalledTimes(1)
  await act(async () => failure.preparations.shift()!())
  await waitFor(() => expect(transport.play).toHaveBeenCalledTimes(2))
  expect(next.source).toHaveBeenCalled()
  expect(failure.transports).toHaveLength(1)
})

it.each(['pause', 'seek', 'refresh', 'flow', 'suspended', 'marker', 'hidden', 'unmount'])(
  'a %s during rebuffer defeats all late prepare/frame completions',
  async (action) => {
    const { controller, props, view, transport } = await playLive()
    failure.holdPrepare = true
    view.rerender(<ClipBrowserDraftPreview {...props} local={undefined} />)
    const next = liveRuntime(2, controller.signal)
    view.rerender(<ClipBrowserDraftPreview {...props} local={next} />)
    await waitFor(() => expect(failure.preparations).toHaveLength(1))
    if (action === 'pause') fireEvent.click(screen.getByRole('button', { name: '일시 정지' }))
    if (action === 'seek')
      view.rerender(<ClipBrowserDraftPreview {...props} local={next} timeMs={1000} />)
    if (action === 'refresh')
      fireEvent.click(screen.getByRole('button', { name: '미리보기 새로고침' }))
    if (action === 'flow') fireEvent.click(screen.getByRole('button', { name: '흐름 보기' }))
    if (action === 'suspended')
      view.rerender(<ClipBrowserDraftPreview {...props} local={next} suspended />)
    if (action === 'marker') act(() => controller.abort())
    if (action === 'hidden') {
      vi.spyOn(document, 'hidden', 'get').mockReturnValue(true)
      fireEvent(document, new Event('visibilitychange'))
    }
    if (action === 'unmount') view.unmount()
    await act(async () => failure.preparations.splice(0).forEach((finish) => finish()))
    await act(async () => failure.frames.splice(0).forEach((finish) => finish()))
    expect(transport.play).toHaveBeenCalledTimes(1)
    expect(transport.running).toBe(false)
  },
)

it('refuses cached sound recovery if a no-caption fresh used original is missing', async () => {
  const { controller, props, view, transport } = await playLive()
  view.rerender(<ClipBrowserDraftPreview {...props} local={undefined} />)
  const next = liveRuntime(2, controller.signal)
  next.source = vi.fn(async () => {
    throw new Error('CLIP_SOURCE_MISSING')
  })
  view.rerender(<ClipBrowserDraftPreview {...props} local={next} />)
  await screen.findByRole('alert')
  expect(next.source).toHaveBeenCalled()
  expect(transport.play).toHaveBeenCalledTimes(1)
  expect(transport.running).toBe(false)
})

it('keeps nested fingerprint changes and revision rollback incompatible', async () => {
  const { controller, props, view, transport } = await playLive()
  view.rerender(<ClipBrowserDraftPreview {...props} local={undefined} />)
  const next = liveRuntime(2, controller.signal)
  next.snapshot = {
    ...next.snapshot,
    ink: { fingerprint: 'different' },
  } as unknown as BrowserCompositionSnapshot
  view.rerender(<ClipBrowserDraftPreview {...props} local={next} />)
  await waitFor(() => expect(screen.getByRole('button', { name: '재생' })).toBeEnabled())
  expect(transport.play).toHaveBeenCalledTimes(1)
})

it('preserves one original intent through split project/correction revision renders', async () => {
  const { controller, props, view, transport } = await playLive()
  failure.holdPrepare = true
  view.rerender(<ClipBrowserDraftPreview {...props} local={undefined} />)
  const middle = liveRuntime(2, controller.signal)
  middle.snapshot = { ...middle.snapshot, projectRevision: 1, snapshotFingerprint: 'middle' }
  view.rerender(<ClipBrowserDraftPreview {...props} local={middle} />)
  await waitFor(() => expect(failure.preparations).toHaveLength(1))
  view.rerender(<ClipBrowserDraftPreview {...props} local={undefined} />)
  const final = liveRuntime(2, controller.signal)
  view.rerender(<ClipBrowserDraftPreview {...props} local={final} />)
  await waitFor(() => expect(failure.preparations).toHaveLength(2))
  await act(async () => failure.preparations.splice(0).forEach((finish) => finish()))
  await waitFor(() => expect(transport.play).toHaveBeenCalledTimes(2))
  expect(middle.source).not.toHaveBeenCalled()
  expect(final.source).toHaveBeenCalled()
  expect(failure.transports).toHaveLength(1)
})

it.each([
  [3, 3],
  [3, 1],
  [1, 3],
])(
  'a committed (%i, %i) high-water cancels lower revisions without later revival',
  async (project, plan) => {
    const { controller, props, view, transport } = await playLive()
    failure.holdPrepare = true
    view.rerender(<ClipBrowserDraftPreview {...props} local={undefined} />)
    const higher = liveRuntime(3, controller.signal)
    higher.snapshot = {
      ...higher.snapshot,
      projectRevision: project,
      planRevision: plan,
      snapshotFingerprint: `higher-${project}-${plan}`,
    }
    view.rerender(<ClipBrowserDraftPreview {...props} local={higher} />)
    await waitFor(() => expect(failure.preparations).toHaveLength(1))
    view.rerender(<ClipBrowserDraftPreview {...props} local={undefined} />)
    const lower = liveRuntime(2, controller.signal)
    view.rerender(<ClipBrowserDraftPreview {...props} local={lower} />)
    await waitFor(() => expect(failure.preparations).toHaveLength(2))
    await act(async () => {
      failure.holdPrepare = false
      failure.preparations.splice(0).forEach((finish) => finish())
    })
    await waitFor(() => expect(failure.events).toContain('draw:revision-2'))
    expect(transport.play).toHaveBeenCalledTimes(1)
    view.rerender(<ClipBrowserDraftPreview {...props} local={higher} />)
    await waitFor(() => expect(failure.events).toContain(`draw:higher-${project}-${plan}`))
    expect(transport.play).toHaveBeenCalledTimes(1)
    expect(transport.running).toBe(false)
    fireEvent.click(screen.getByRole('button', { name: '재생' }))
    await waitFor(() => expect(transport.running).toBe(true))
  },
)

it.each(['access', 'frame', 'play'])(
  'Pause wins while the fresh %s is pending; manual later Play still works',
  async (stage) => {
    const { controller, props, view, transport } = await playLive()
    let finishAccess!: () => void
    const next = liveRuntime(2, controller.signal)
    if (stage === 'access')
      next.source = vi.fn(
        () =>
          new Promise<{ kind: 'url'; url: string }>((resolve) => {
            finishAccess = () => resolve({ kind: 'url', url: 'https://owned.test/current' })
          }),
      )
    if (stage === 'frame') failure.holdFrame = true
    if (stage === 'play') failure.holdPlay = true
    view.rerender(<ClipBrowserDraftPreview {...props} local={undefined} />)
    view.rerender(<ClipBrowserDraftPreview {...props} local={next} />)
    if (stage === 'access') await waitFor(() => expect(next.source).toHaveBeenCalled())
    if (stage === 'frame') await waitFor(() => expect(failure.frames.length).toBeGreaterThan(0))
    if (stage === 'play') await waitFor(() => expect(failure.playbacks).toHaveLength(1))
    fireEvent.click(screen.getByRole('button', { name: '일시 정지' }))
    await act(async () => {
      if (finishAccess) finishAccess()
      failure.frames.splice(0).forEach((finish) => finish())
      failure.playbacks.splice(0).forEach((finish) => finish())
    })
    expect(transport.running).toBe(false)
    expect(transport.play).toHaveBeenCalledTimes(stage === 'play' ? 2 : 1)
    failure.holdFrame = false
    failure.holdPlay = false
    await waitFor(() => expect(screen.getByRole('button', { name: '재생' })).toBeEnabled())
    fireEvent.click(screen.getByRole('button', { name: '재생' }))
    await waitFor(() => expect(transport.running).toBe(true))
  },
)

it('does not automatically borrow a changed audio plan or an older revision', async () => {
  const { controller, props, view, transport } = await playLive()
  view.rerender(<ClipBrowserDraftPreview {...props} local={undefined} />)
  const next = liveRuntime(0, controller.signal)
  view.rerender(<ClipBrowserDraftPreview {...props} local={next} />)
  await waitFor(() => expect(screen.getByRole('button', { name: '재생' })).toBeEnabled())
  expect(transport.play).toHaveBeenCalledTimes(1)
  fireEvent.click(screen.getByRole('button', { name: '재생' }))
  await waitFor(() => expect(transport.running).toBe(true))
  const changed = liveRuntime(2, controller.signal)
  changed.snapshot = {
    ...changed.snapshot,
    plan: { ...changed.snapshot.plan, sourceVolumePermille: 500 },
  }
  view.rerender(<ClipBrowserDraftPreview {...props} local={changed} />)
  await waitFor(() => expect(screen.getByRole('button', { name: '재생' })).toBeEnabled())
  expect(failure.transports.at(-1)!.play).not.toHaveBeenCalled()
  expect(transport.running).toBe(false)
})

it('keeps only the live runtime after StrictMode replay and cannot resume after unmount', async () => {
  const { props, view } = await playLive(true)
  const active = failure.transports.find((transport) => transport.running)!
  failure.holdPrepare = true
  view.rerender(<ClipBrowserDraftPreview {...props} local={undefined} />)
  view.unmount()
  await act(async () => failure.preparations.splice(0).forEach((finish) => finish()))
  expect(active.running).toBe(false)
  expect(failure.transports.every((transport) => !transport.running)).toBe(true)
  expect(failure.transports.every((transport) => transport.dispose.mock.calls.length === 1)).toBe(
    true,
  )
})

it('keeps the exact continuation frame queued behind an aborted old frame while the previous controlled value is unchanged', async () => {
  enableLive()
  const controller = new AbortController(),
    runtime = liveRuntime(1, controller.signal)
  const change = vi.fn()
  const props = { ...liveProps(runtime), timeMs: 0, onTimeChange: change }
  const view = render(<ClipBrowserDraftPreview {...props} />)
  await waitFor(() => expect(screen.getByRole('button', { name: '재생' })).toBeEnabled())
  fireEvent.click(screen.getByRole('button', { name: '재생' }))
  await waitFor(() => expect(failure.transports[0]!.running).toBe(true))
  await waitFor(() => expect(change).toHaveBeenCalledWith(0))
  failure.holdFrame = true
  failure.transports[0]!.position = 3533
  await waitFor(() => expect(failure.frames).toHaveLength(1))
  view.rerender(<ClipBrowserDraftPreview {...props} local={undefined} />)
  const next = liveRuntime(2, controller.signal)
  view.rerender(<ClipBrowserDraftPreview {...props} local={next} />)
  await waitFor(() => expect(next.source).toHaveBeenCalled())
  await act(async () => failure.frames.shift()!())
  await waitFor(() => expect(failure.frames).toHaveLength(1))
  await act(async () => failure.frames.shift()!())
  await waitFor(() => expect(failure.transports[0]!.play).toHaveBeenCalledTimes(2))
  expect(failure.transports[0]!.play).toHaveBeenLastCalledWith(3533)
  expect(failure.events.filter((event) => event === 'close:revision-1').length).toBeGreaterThan(0)
})

it('a parent timeMs-only seek AFTER the fresh target is pinned cancels it and all late frames', async () => {
  const { controller, props, view, transport } = await playLive()
  const change = vi.fn()
  const next = liveRuntime(2, controller.signal)
  failure.holdFrame = true
  view.rerender(<ClipBrowserDraftPreview {...props} local={undefined} onTimeChange={change} />)
  view.rerender(<ClipBrowserDraftPreview {...props} local={next} onTimeChange={change} />)
  await waitFor(() => expect(failure.frames).toHaveLength(1))
  expect(change).toHaveBeenLastCalledWith(3533)
  view.rerender(
    <ClipBrowserDraftPreview {...props} local={next} timeMs={1000} onTimeChange={change} />,
  )
  const callsAfterSeek = change.mock.calls.length
  await act(async () => failure.frames.splice(0).forEach((finish) => finish()))
  await act(async () => failure.frames.splice(0).forEach((finish) => finish()))
  expect(change.mock.calls.slice(callsAfterSeek)).not.toContainEqual([3533])
  expect(transport.play).toHaveBeenCalledTimes(1)
  expect(transport.running).toBe(false)
  failure.holdFrame = false
  await waitFor(() => expect(screen.getByRole('button', { name: '재생' })).toBeEnabled())
  fireEvent.click(screen.getByRole('button', { name: '재생' }))
  await waitFor(() => expect(transport.play).toHaveBeenCalledTimes(2))
  expect(transport.play).toHaveBeenLastCalledWith(1000)
})

it('does not continue if the native clock reaches EOF while the old owner pauses', async () => {
  const { controller, runtime, props, view, transport } = await playLive()
  const duration = (runtime.snapshot.frameCount * 1000) / 30
  transport.position = duration - 1
  const pausing = transport as unknown as { pause: ReturnType<typeof vi.fn> }
  pausing.pause.mockImplementation(() => {
    transport.running = false
    transport.position = duration
  })
  view.rerender(<ClipBrowserDraftPreview {...props} local={undefined} />)
  view.rerender(<ClipBrowserDraftPreview {...props} local={liveRuntime(2, controller.signal)} />)
  await waitFor(() => expect(screen.getByRole('button', { name: '재생' })).toBeEnabled())
  expect(transport.play).toHaveBeenCalledTimes(1)
  expect(transport.running).toBe(false)
})
