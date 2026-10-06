import { create } from '@bufbuild/protobuf'
import { createRouterTransport } from '@connectrpc/connect'
import { act, renderHook } from '@testing-library/react'
import { beforeEach, expect, it, vi } from 'vitest'
import {
  ClipEditingStateSchema,
  ClipGenerationService,
  ClipProjectSchema,
  ClipRenderService,
} from '@/shared/api'
import { clipPlanToProto } from '@/entities/clip-plan'
import { clipTimelineFixture } from '@/test/clip-editing'
import { createTestQueryClient, withProviders } from '@/test/session'
import { useBrowserRender } from './useBrowserRender'
import { BrowserUploadPendingError } from './run-local-render'
import type { VerifiedBrowserResult } from './store-result'
const { run, put, createUrl, revokeUrl } = vi.hoisted(() => ({
  run: vi.fn(),
  put: vi.fn(),
  createUrl: vi.fn(() => 'blob:verified-local'),
  revokeUrl: vi.fn(),
}))
vi.mock('./run-render', async (original) => ({
  ...(await original<typeof import('./run-render')>()),
  runBrowserRender: run,
}))
vi.mock('@/shared/lib/upload', () => ({ putBlobWithProgress: put }))
beforeEach(() => {
  vi.clearAllMocks()
  vi.stubGlobal(
    'URL',
    Object.assign(URL, { createObjectURL: createUrl, revokeObjectURL: revokeUrl }),
  )
})

function harness() {
  const editing = clipTimelineFixture()
  const artifact: VerifiedBrowserResult = {
    renderId: 'render',
    file: new Blob(['verified result']),
    verdict: { passed: true, measurements: {} } as VerifiedBrowserResult['verdict'],
    dispose: vi.fn(async () => {}),
  }
  const cancelled = vi.fn(async () => ({ cancelled: true })),
    prepare = vi.fn(async () => ({ putUrl: 'private-upload', headers: {} })),
    report = vi.fn(async () => ({ passed: true, notices: [] })),
    complete = vi.fn(async () => ({
      project: create(ClipProjectSchema, {
        id: 'clip',
        ratio: 'vertical',
        editPlanRevision: 3,
        renderedPlanRevision: 3,
      }),
    }))
  const transport = createRouterTransport((router) => {
    router.service(ClipRenderService, {
      cancelClipBrowserRender: cancelled,
      prepareClipRenderUpload: prepare,
      reportClipRenderVerdict: report,
      completeClipRenderUpload: complete,
    })
    router.rpc(ClipGenerationService.method.getClipProject, () => ({
      project: create(ClipProjectSchema, {
        id: 'clip',
        title: 'Test',
        ratio: 'vertical',
        editPlanRevision: 3,
        renderedPlanRevision: 1,
        editing: create(ClipEditingStateSchema, {
          ...editing,
          plan: clipPlanToProto(editing.plan),
        }),
      }),
    }))
    router.rpc(ClipGenerationService.method.listClipProjects, () => ({ projects: [] }))
  })
  run.mockImplementation(async (input) => {
    input.onLocalReady(artifact)
    throw new BrowserUploadPendingError(artifact, new Error('upload disconnected'))
  })
  const view = renderHook(
    ({ ownerId, revision, finalized }) => useBrowserRender(ownerId, 'clip', revision, finalized),
    {
      initialProps: { ownerId: 'alice', revision: 3, finalized: false },
      wrapper: withProviders(transport, createTestQueryClient()),
    },
  )
  const start = () =>
    act(() =>
      view.result.current.start({
        batchId: 'batch',
        localSources: [],
        resolvePlayback: vi.fn(),
        flush: async () => 3,
      }),
    )
  return { view, artifact, cancelled, prepare, report, complete, start }
}
it('keeps a verified local preview while private upload is pending and retries bytes without encoding again', async () => {
  const f = harness()
  await f.start()
  expect(f.view.result.current.state).toMatchObject({
    phase: 'upload_pending',
    local: { url: 'blob:verified-local', revision: 3 },
  })
  expect(f.complete).not.toHaveBeenCalled()
  expect(f.artifact.dispose).not.toHaveBeenCalled()
  put.mockResolvedValue(undefined)
  await act(() => f.view.result.current.retry())
  expect(f.prepare).toHaveBeenCalledTimes(1)
  expect(put).toHaveBeenCalledTimes(1)
  expect(f.report).toHaveBeenCalledTimes(1)
  expect(f.complete).toHaveBeenCalledTimes(1)
  expect(f.view.result.current.state).toMatchObject({ phase: 'done', progress: { percent: 100 } })
  expect(f.view.result.current.state.local).toBeUndefined()
  expect(run).toHaveBeenCalledTimes(1)
  expect(put.mock.calls[0]?.[2]).toBe(f.artifact.file)
  expect(f.artifact.dispose).toHaveBeenCalledTimes(1)
  expect(revokeUrl).toHaveBeenCalledWith('blob:verified-local')
})
it('fences a late retry after pagehide and destroys the provisional URL and output', async () => {
  const f = harness()
  await f.start()
  let finish!: () => void
  put.mockImplementation(
    () =>
      new Promise<void>((resolve) => {
        finish = resolve
      }),
  )
  let retry!: Promise<void>
  await act(async () => {
    retry = f.view.result.current.retry()
    await Promise.resolve()
  })
  await vi.waitFor(() => expect(put).toHaveBeenCalled())
  act(() => window.dispatchEvent(new Event('pagehide')))
  await act(async () => {
    finish()
    await retry
  })
  expect(f.report).not.toHaveBeenCalled()
  expect(f.complete).not.toHaveBeenCalled()
  expect(f.cancelled).toHaveBeenCalledTimes(1)
  expect(f.artifact.dispose).toHaveBeenCalledTimes(1)
})
it.each(['owner', 'revision', 'finalized'] as const)(
  'removes local output and cancels promotion on %s replacement',
  async (kind) => {
    const f = harness()
    await f.start()
    f.view.rerender({
      ownerId: kind === 'owner' ? 'bob' : 'alice',
      revision: kind === 'revision' ? 4 : 3,
      finalized: kind === 'finalized',
    })
    expect(f.view.result.current.state.local).toBeUndefined()
    await vi.waitFor(() => expect(f.artifact.dispose).toHaveBeenCalledTimes(1))
    expect(revokeUrl).toHaveBeenCalledWith('blob:verified-local')
    await act(() => f.view.result.current.retry())
    expect(put).not.toHaveBeenCalled()
  },
)
