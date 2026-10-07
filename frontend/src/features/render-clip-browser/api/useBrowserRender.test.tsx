import { create } from '@bufbuild/protobuf'
import { createRouterTransport } from '@connectrpc/connect'
import { act, renderHook } from '@testing-library/react'
import { expect, it, vi } from 'vitest'
import {
  ClipEditingStateSchema,
  ClipGenerationService,
  ClipProjectSchema,
  ClipRenderService,
  GenerationService,
  StartClipRenderResponseSchema,
} from '@/shared/api'
import { clipPlanToProto } from '@/entities/clip-plan'
import { clipTimelineFixture } from '@/test/clip-editing'
import { createTestQueryClient, withProviders } from '@/test/session'
import { useBrowserRender } from './useBrowserRender'

function harness(savedRevision: number) {
  const started = vi.fn(() => create(StartClipRenderResponseSchema, { renderId: 'render' }))
  const editing = clipTimelineFixture()
  const transport = createRouterTransport((router) => {
    router.service(ClipRenderService, { startClipRender: started })
    router.rpc(ClipGenerationService.method.getClipProject, () => ({
      project: create(ClipProjectSchema, {
        id: 'clip',
        title: 'Test',
        ratio: 'vertical',
        editPlanRevision: savedRevision,
        renderedPlanRevision: 1,
        editing: create(ClipEditingStateSchema, {
          ...editing,
          plan: clipPlanToProto(editing.plan),
        }),
      }),
    }))
    router.rpc(ClipGenerationService.method.listClipProjects, () => ({ projects: [] }))
  })
  const view = renderHook(() => useBrowserRender('alice', 'clip'), {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  const start = (flush: () => Promise<number>) =>
    act(() =>
      view.result.current.start({
        batchId: 'batch',
        localSources: [],
        resolvePlayback: vi.fn(),
        flush,
      }),
    )
  return { view, started, start }
}

// CLIP-39, CLIP-188: a browser render starts only from edits that reached the server. A flush
// that refuses — a slot holding words it cannot draw, a save the server turned down — starts
// nothing; the render reports the failure instead.
it('starts no browser render when the edits could not be flushed', async () => {
  const { view, started, start } = harness(3)
  await start(() => Promise.reject(new Error('Invalid clip regions')))
  expect(view.result.current.state.phase).toBe('failed')
  expect(started).not.toHaveBeenCalled()
})

// CLIP-152: a plan saved past the flushed revision — a slot or preset save landing after it —
// is not rendered as that revision's; the render refuses as a conflict before any work.
it('renders no revision older than the one the server holds', async () => {
  const { view, started, start } = harness(4)
  await start(() => Promise.resolve(3))
  expect(view.result.current.state).toMatchObject({
    phase: 'failed',
    failure: { reason: 'CLIP_PLAN_CONFLICT' },
  })
  expect(started).not.toHaveBeenCalled()
})

// Unknown legacy source audio metadata must not start a native sampling fallback.
it('refuses unknown source identity before legacy admission or native sampling', async () => {
  const editing = clipTimelineFixture()
  const cancelled = vi.fn(() => ({ cancelled: true }))
  const transport = createRouterTransport((router) => {
    router.service(ClipRenderService, {
      startClipRender: () =>
        create(StartClipRenderResponseSchema, { renderId: 'render', jobId: 'sampling' }),
      cancelClipBrowserRender: cancelled,
    })
    router.rpc(GenerationService.method.getGeneration, () => ({
      job: {
        id: 'sampling',
        kind: 'sample_browser_render',
        status: 'failed',
        failure: { reason: 'CLIP_SOURCE_UNAVAILABLE', params: {} },
      },
    }))
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
  const view = renderHook(() => useBrowserRender('alice', 'clip'), {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  await act(() =>
    view.result.current.start({
      batchId: 'batch',
      localSources: [],
      resolvePlayback: vi.fn(),
      flush: () => Promise.resolve(3),
    }),
  )
  expect(view.result.current.state).toMatchObject({
    phase: 'failed',
    failure: { reason: 'CLIP_PROCESSING_FAILED' },
  })
  expect(cancelled).not.toHaveBeenCalled()
})
