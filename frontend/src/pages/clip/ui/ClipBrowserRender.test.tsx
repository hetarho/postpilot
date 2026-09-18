import { create } from '@bufbuild/protobuf'
import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { ClipSourceBatchSchema } from '@/shared/api'
import { putBlobWithProgress } from '@/shared/lib/upload'
import type { BrowserVideoTrack } from '@/entities/clip-project'
import { observedClipFixture } from '@/test/clip-observations'
import { clipTimelineFixture } from '@/test/clip-editing'
import { renderAppAt } from '@/test/app'
import { discardClipDraftQueues } from '@/features/edit-clip-project'

const media = vi.hoisted(() => ({ video: vi.fn(), dispose: vi.fn() }))
vi.mock('@/features/render-clip-browser/api/run-render', async (original) => {
  const actual = await original<typeof import('@/features/render-clip-browser/api/run-render')>()
  const { storeBrowserResult, createBrowserResultStore } =
    await import('@/features/render-clip-browser/api/store-result')
  return {
    ...actual,
    browserRenderOperations: (transport: Parameters<typeof actual.browserRenderOperations>[0]) =>
      ({
        ...actual.browserRenderOperations(transport),
        prepare: async () => ({ assets: [], width: 1080, height: 1920, dispose: media.dispose }),
        video: media.video,
        audio: async () => undefined,
        store: (id, video, audio, input, signal, progress) =>
          storeBrowserResult(
            id,
            video,
            audio,
            input.ratio,
            input.plan.durationMs,
            createBrowserResultStore(transport),
            signal,
            progress,
            async () => new Blob(['encoded-mp4'], { type: 'video/mp4' }),
          ),
      }) satisfies ReturnType<typeof actual.browserRenderOperations>,
  }
})
vi.mock('@/shared/lib/upload', async (original) => ({
  ...(await original<object>()),
  putBlobWithProgress: vi.fn(),
}))

function deferred<T>() {
  let resolve!: (value: T) => void, reject!: (reason: unknown) => void
  const promise = new Promise<T>((yes, no) => {
    resolve = yes
    reject = no
  })
  return { promise, resolve, reject }
}
let encoded: ReturnType<typeof deferred<BrowserVideoTrack>>
let uploaded: ReturnType<typeof deferred<void>>
let workerSignal: AbortSignal | undefined
let uploadSignal: AbortSignal | undefined
let uploadProgress: (percent: number) => void
let track: BrowserVideoTrack

beforeEach(() => {
  vi.stubGlobal('VideoEncoder', { isConfigSupported: async () => ({ supported: true }) })
  vi.stubGlobal('AudioEncoder', { isConfigSupported: async () => ({ supported: true }) })
  encoded = deferred()
  uploaded = deferred()
  workerSignal = uploadSignal = undefined
  track = {
    config: { codec: 'avc1.640028', width: 1080, height: 1920, framerate: 30 },
    decoderConfig: { codec: 'avc1.640028', codedWidth: 1080, codedHeight: 1920 },
    frameCount: 594,
    durationUs: 19800000,
    chunks: Array.from({ length: 594 }, (_, i) => ({
      data: new Uint8Array([1]),
      type: i === 0 ? 'key' : 'delta',
      timestamp: Math.round((i * 1e6) / 30),
      duration: Math.round(1e6 / 30),
    })),
  }
  media.dispose.mockClear()
  media.video.mockImplementation((_input, _local, _resolve, signal: AbortSignal) => {
    workerSignal = signal
    const cancel = () => encoded.reject(new DOMException('Stopped', 'AbortError'))
    signal.addEventListener('abort', cancel, { once: true })
    return {
      result: encoded.promise,
      cancel,
      progress: {
        async *[Symbol.asyncIterator]() {
          yield { completedFrames: 25, totalFrames: 100 }
          await encoded.promise
          yield { completedFrames: 100, totalFrames: 100 }
        },
      },
    }
  })
  vi.mocked(putBlobWithProgress).mockImplementation(
    async (_url, _headers, _blob, progress, signal) => {
      uploadProgress = progress
      uploadSignal = signal
      signal?.addEventListener(
        'abort',
        () => uploaded.reject(new DOMException('Stopped', 'AbortError')),
        { once: true },
      )
      await uploaded.promise
      progress(100)
    },
  )
})
afterEach(() => {
  vi.unstubAllGlobals()
  discardClipDraftQueues()
  vi.clearAllMocks()
})

async function mount() {
  const project = observedClipFixture()
  project.editing = clipTimelineFixture()
  project.targetDurationMs = 19800
  project.result!.renderKind = 'browser'
  project.editing.plan.sourceAudio = project.editing.plan.cuts.map((c) => ({
    sourceId: c.sourceId,
    fingerprint: c.fingerprint,
    retainOriginalAudio: false,
  }))
  const batch = create(ClipSourceBatchSchema, {
    id: 'retained',
    projectId: project.id,
    state: 'ready',
    current: true,
    expiresAt: '2099-01-01T00:00:00Z',
    sources: project.editing.sources.map((source) => ({
      id: source.id,
      state: 'ready',
      availability: 'available',
      retentionExpiresAt: '2099-01-01T00:00:00Z',
      metadata: { ...source, contentType: 'video/mp4', bytes: 5n },
    })),
  })
  const calls: string[] = []
  const view = renderAppAt('/clips/project', {
    user: { id: 'alice' },
    clips: { projects: [project], retainedBatches: [batch], calls },
  })
  const button = await screen.findByRole('button', { name: '다시 렌더 · 브라우저' })
  await waitFor(() => expect(button).toBeEnabled())
  await userEvent.click(button)
  await screen.findByText('브라우저에서 영상 만드는 중')
  await waitFor(() => expect(workerSignal).toBeDefined())
  return { ...view, calls }
}

it('keeps ② interactive and reports one monotonic scale from encoding through durable storage', async () => {
  const { calls } = await mount()
  await waitFor(() =>
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '20'),
  )
  expect(screen.queryByRole('region', { name: '클립 작업 진행' })).not.toBeInTheDocument()
  expect(screen.getByRole('tab', { name: '클립 다듬기' })).toHaveAttribute('aria-selected', 'true')
  expect(screen.getByLabelText('요청 내용')).toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: '참고 자료' }))
  expect(await screen.findByRole('dialog', { name: '참고 자료' })).toBeInTheDocument()
  await userEvent.keyboard('{Escape}')
  await userEvent.click(screen.getByRole('tab', { name: '클립 생성' }))
  expect(screen.getByLabelText('클립 제목')).toBeInTheDocument()
  await userEvent.click(screen.getByRole('tab', { name: '클립 다듬기' }))
  await act(async () => encoded.resolve(track))
  await screen.findByText('완성된 영상 저장 중')
  expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '80')
  await waitFor(() => expect(uploadSignal).toBeDefined())
  act(() => uploadProgress(50))
  expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '89.5')
  expect(screen.getByRole('link', { name: '렌더 1 다운로드' })).toHaveAttribute(
    'href',
    'https://example.test/download.mp4',
  )
  expect(calls).not.toContain('CompleteClipRenderUpload')
  await act(async () => uploaded.resolve())
  await screen.findByText('브라우저 렌더를 저장했어요.')
  await waitFor(() =>
    expect(screen.getByRole('link', { name: '렌더 1 다운로드' })).toHaveAttribute(
      'href',
      'https://private.test/browser-download.mp4',
    ),
  )
  expect(calls.indexOf('PrepareClipRenderUpload')).toBeLessThan(
    calls.indexOf('ReportClipRenderVerdict'),
  )
  expect(calls.indexOf('ReportClipRenderVerdict')).toBeLessThan(
    calls.indexOf('CompleteClipRenderUpload'),
  )
  expect(calls).not.toContain('CancelClipBrowserRender')
  expect(media.dispose).toHaveBeenCalledOnce()
})

it.each(['encoding', 'storing'])(
  'cancels %s and preserves the plan and previous result',
  async (stage) => {
    const { calls } = await mount()
    if (stage === 'storing') {
      await act(async () => encoded.resolve(track))
      await waitFor(() => expect(uploadSignal).toBeDefined())
    }
    await userEvent.click(screen.getByRole('button', { name: '브라우저 렌더 취소' }))
    await screen.findByText('브라우저 렌더를 취소했어요.')
    expect(workerSignal?.aborted).toBe(true)
    if (stage === 'storing') expect(uploadSignal?.aborted).toBe(true)
    expect(calls).toContain('CancelClipBrowserRender')
    expect(calls).not.toContain('CompleteClipRenderUpload')
    expect(screen.getByRole('heading', { name: '컷·자막 수정' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '렌더 1 다운로드' })).toHaveAttribute(
      'href',
      'https://example.test/download.mp4',
    )
    expect(media.dispose).toHaveBeenCalledOnce()
  },
)

it('ends the run when the page unmounts without opening a leave dialog', async () => {
  const { router, calls } = await mount()
  await act(async () => {
    await router.navigate({ to: '/clips' })
  })
  await waitFor(() => expect(calls).toContain('CancelClipBrowserRender'))
  expect(workerSignal?.aborted).toBe(true)
  await waitFor(() => expect(media.dispose).toHaveBeenCalledOnce())
  expect(calls).not.toContain('CompleteClipRenderUpload')
})

it('shows storage failure beside the render controls and leaves the old download available', async () => {
  const { calls } = await mount()
  await act(async () => encoded.resolve(track))
  await waitFor(() => expect(uploadSignal).toBeDefined())
  await act(async () => uploaded.reject(new Error('storage unavailable')))
  await screen.findByText('브라우저 렌더를 완료하지 못했어요.')
  expect(screen.getByRole('link', { name: '렌더 1 다운로드' })).toHaveAttribute(
    'href',
    'https://example.test/download.mp4',
  )
  expect(calls).not.toContain('CompleteClipRenderUpload')
  expect(calls).toContain('CancelClipBrowserRender')
})
