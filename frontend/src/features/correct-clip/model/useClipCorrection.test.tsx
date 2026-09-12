import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { Code, createRouterTransport } from '@connectrpc/connect'
import { ClipService, ClipProjectSchema, ClipEditingStateSchema } from '@/shared/api'
import {
  clipPlanToProto,
  toClipEditingState,
  toClipProject,
  type ClipProject,
} from '@/entities/clip-project'
import { clipEditingFixture } from '@/test/clip-editing'
import { connectAppError } from '@/test/app-error'
import { createTestQueryClient, withProviders } from '@/test/session'
import { CLIP_TIMELINE } from '@/shared/config'
import { useClipCorrection } from './useClipCorrection'

function setup() {
  const editing = clipEditingFixture()
  const wire = () =>
    create(ClipProjectSchema, {
      id: 'clip',
      title: 'Test',
      ratio: 'vertical',
      editPlanRevision: revision,
      renderedPlanRevision: 1,
      editing: create(ClipEditingStateSchema, { ...editing, plan: clipPlanToProto(editing.plan) }),
    })
  let revision = 1,
    fail = false,
    hold = false
  let release: (() => void) | undefined
  const writes: number[] = []
  const transport = createRouterTransport((router) => {
    router.rpc(ClipService.method.getClipProject, () => ({ project: wire() }))
    router.rpc(ClipService.method.saveClipEditPlan, async (req) => {
      writes.push(req.expectedRevision)
      if (hold)
        await new Promise<void>((resolve) => {
          release = resolve
        })
      if (fail) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
      if (req.expectedRevision !== revision)
        throw connectAppError('CLIP_PLAN_CONFLICT', Code.Aborted)
      editing.plan = toClipEditingState(
        create(ClipEditingStateSchema, { ...editing, plan: req.plan }),
      ).plan
      revision++
      return { project: wire() }
    })
  })
  const project = toClipProject(wire())
  const view = renderHook(
    ({ project }: { project: ClipProject }) => useClipCorrection('alice', project),
    { wrapper: withProviders(transport, createTestQueryClient()), initialProps: { project } },
  )
  return {
    ...view,
    writes,
    project,
    setFail: (value: boolean) => {
      fail = value
    },
    setHold: (value: boolean) => {
      hold = value
    },
    release: () => release?.(),
    remote: () => {
      revision++
      return toClipProject(wire())
    },
  }
}
beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())
const tick = async (ms: number = CLIP_TIMELINE.autosaveMs) => {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms)
  })
}

it('debounces edits, keeps typing during save and uses the accepted revision for the next save and undo', async () => {
  const view = setup()
  await tick()
  expect(view.writes).toEqual([])
  view.setHold(true)
  act(() =>
    view.result.current.change(
      { type: 'cut', id: 'cut-a', patch: { volumePermille: 800 } },
      'volume',
    ),
  )
  await tick()
  expect(view.writes).toEqual([1])
  await tick(1)
  expect(view.result.current.pending).toBe(true)
  act(() =>
    view.result.current.change(
      { type: 'cut', id: 'cut-a', patch: { volumePermille: 600 } },
      'volume',
    ),
  )
  expect(view.result.current.draft.cuts[0].volumePermille).toBe(600)
  view.setHold(false)
  await act(async () => view.release())
  await tick()
  expect(view.writes).toEqual([1, 2])
  expect(view.result.current.dirty).toBe(false)
  act(() => view.result.current.dispatch({ type: 'undo' }))
  await tick()
  expect(view.writes).toEqual([1, 2, 3])
  expect(view.result.current.revision).toBe(4)
})

it('retains a failed draft without retry loops and explicitly retries it', async () => {
  const view = setup()
  view.setFail(true)
  act(() =>
    view.result.current.change({ type: 'cut', id: 'cut-a', patch: { volumePermille: 500 } }),
  )
  await tick()
  await tick(5000)
  expect(view.writes).toEqual([1])
  expect(view.result.current.draft.cuts[0].volumePermille).toBe(500)
  expect(view.result.current.failure?.reason).toBe('NETWORK_UNAVAILABLE')
  view.setFail(false)
  await act(async () => {
    await view.result.current.save()
  })
  expect(view.result.current.dirty).toBe(false)
})

it('holds a conflicting draft until explicit reapply and never replaces its fields from polling', async () => {
  const view = setup()
  act(() =>
    view.result.current.change({ type: 'cut', id: 'cut-a', patch: { volumePermille: 400 } }),
  )
  const remote = view.remote()
  view.rerender({ project: remote })
  await tick(5000)
  expect(view.writes).toEqual([])
  expect(view.result.current.failure?.reason).toBe('CLIP_PLAN_CONFLICT')
  expect(view.result.current.draft.cuts[0].volumePermille).toBe(400)
  await act(async () => view.result.current.reapply())
  await tick()
  expect(view.writes).toEqual([2])
  expect(view.result.current.draft.cuts[0].volumePermille).toBe(400)
})

it('does not save an invalid authored interval and keeps it available for repair', async () => {
  const view = setup()
  act(() => view.result.current.change({ type: 'cut', id: 'cut-a', patch: { startMs: 20000 } }))
  await tick(5000)
  expect(view.result.current.validation?.valid).toBe(false)
  expect(view.result.current.draft.cuts[0].startMs).toBe(20000)
  expect(view.writes).toEqual([])
})
