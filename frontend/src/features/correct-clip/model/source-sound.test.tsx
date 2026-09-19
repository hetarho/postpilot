import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { Code, createRouterTransport } from '@connectrpc/connect'
import {
  ClipGenerationService,
  ClipPlanService,
  ClipSourceService,
  ClipProjectSchema,
  ClipEditingStateSchema,
  ClipSourceBatchSchema,
} from '@/shared/api'
import { clipPlanToProto, toClipEditingState, withSourceSound } from '@/entities/clip-plan'
import { toClipProject } from '@/entities/clip-project'
import { clipTimelineFixture } from '@/test/clip-editing'
import { connectAppError } from '@/test/app-error'
import { createTestQueryClient, withProviders } from '@/test/session'
import { CLIP_TIMELINE } from '@/entities/clip-design'
import { useClipCorrection } from './useClipCorrection'

function setup(preplan = false) {
  let editing = preplan ? undefined : clipTimelineFixture()
  if (editing)
    editing.plan.sourceAudio = editing.plan.cuts.map((c) => ({
      sourceId: c.sourceId,
      fingerprint: c.fingerprint,
      retainOriginalAudio: false,
    }))
  let revision = preplan ? 0 : 1
  let fail = false
  let hold: 'sound' | 'plan' | undefined
  let release: (() => void) | undefined
  const sources = ['a', 'b', 'unused'].map((id) => ({
    batchId: 'batch',
    sourceId: id,
    fingerprint: id.repeat(64),
    retainOriginalAudio: false,
  }))
  const enabled = new Map(sources.map((s) => [s.sourceId, false]))
  const batch = () =>
    create(ClipSourceBatchSchema, {
      id: 'batch',
      projectId: 'clip',
      current: true,
      state: 'ready',
      expiresAt: '2099-01-01T00:00:00Z',
      sources: sources.map((s) => ({
        id: s.sourceId,
        state: 'ready',
        metadata: { fingerprint: s.fingerprint, filename: s.sourceId + '.mp4' },
        retainOriginalAudio: enabled.get(s.sourceId),
      })),
    })
  const wire = () =>
    create(ClipProjectSchema, {
      id: 'clip',
      ratio: 'vertical',
      editPlanRevision: revision,
      renderedPlanRevision: preplan ? 0 : 1,
      editing: editing ? { ...editing, plan: clipPlanToProto(editing.plan) } : undefined,
    })
  const writes: { kind: string; revision: number; enabled?: boolean; sourceId?: string }[] = []
  const wait = async (kind: typeof hold) => {
    if (hold === kind)
      await new Promise<void>((r) => {
        release = r
      })
    if (fail) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
  }
  const transport = createRouterTransport((router) => {
    router.rpc(ClipGenerationService.method.getClipProject, () => ({ project: wire() }))
    router.rpc(ClipSourceService.method.getClipSources, () => ({ batches: [batch()] }))
    router.rpc(ClipSourceService.method.setClipSourceOriginalSound, async (req) => {
      writes.push({
        kind: 'sound',
        revision: req.expectedRevision,
        enabled: req.retainOriginalAudio,
        sourceId: req.sourceId,
      })
      await wait('sound')
      if (req.expectedRevision !== revision)
        throw connectAppError('CLIP_PLAN_CONFLICT', Code.Aborted)
      expect(req.expectedFingerprint).toBe(
        sources.find((s) => s.sourceId === req.sourceId)!.fingerprint,
      )
      if (enabled.get(req.sourceId) !== req.retainOriginalAudio) {
        enabled.set(req.sourceId, req.retainOriginalAudio)
        if (editing) {
          if (editing.plan.cuts.some((c) => c.sourceId === req.sourceId))
            editing.plan = withSourceSound(editing.plan, {
              sourceId: req.sourceId,
              fingerprint: req.expectedFingerprint,
              retainOriginalAudio: req.retainOriginalAudio,
            })
          revision++
        }
      }
      return { project: wire(), batch: batch() }
    })
    router.rpc(ClipPlanService.method.saveClipEditPlan, async (req) => {
      writes.push({ kind: 'plan', revision: req.expectedRevision })
      await wait('plan')
      if (req.expectedRevision !== revision)
        throw connectAppError('CLIP_PLAN_CONFLICT', Code.Aborted)
      expect(req.plan?.sourceAudio?.values).not.toContainEqual(
        expect.objectContaining({ sourceId: 'unused' }),
      )
      for (const setting of req.plan?.sourceAudio?.values ?? [])
        expect(setting.retainOriginalAudio).toBe(enabled.get(setting.sourceId))
      editing = toClipEditingState(create(ClipEditingStateSchema, { ...editing, plan: req.plan }))
      revision++
      return { project: wire() }
    })
  })
  const view = renderHook(({ project }) => useClipCorrection('owner', project), {
    initialProps: { project: toClipProject(wire()) },
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  return {
    ...view,
    sources,
    writes,
    remote: () => {
      revision++
      return toClipProject(wire())
    },
    fail: (v: boolean) => {
      fail = v
    },
    hold: (v: typeof hold) => {
      hold = v
    },
    release: () => release?.(),
    stored: () => editing?.plan,
  }
}
beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())
const tick = async () => {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(CLIP_TIMELINE.autosaveMs + 1)
  })
}

it('saves a pre-plan choice at revision zero and keeps a failed intent for explicit retry', async () => {
  const view = setup(true),
    source = view.sources[0]
  expect(view.result.current.soundValue(source)).toBe(false)
  view.fail(true)
  act(() => view.result.current.setSourceSound(source, true))
  expect(view.result.current.soundValue(source)).toBe(true)
  await tick()
  expect(view.writes).toEqual([{ kind: 'sound', revision: 0, sourceId: 'a', enabled: true }])
  expect(view.result.current.soundValue(source)).toBe(false)
  expect(view.result.current.dirty).toBe(true)
  await tick()
  expect(view.writes).toHaveLength(1)
  view.fail(false)
  await act(async () => {
    await view.result.current.save()
  })
  expect(view.result.current.soundValue(source)).toBe(true)
  expect(view.result.current.dirty).toBe(false)
  expect(view.result.current.soundBatch?.sources[0].retainOriginalAudio).toBe(true)
})

it('stales only render and sends inverse semantic writes for saved undo/redo without a plan save', async () => {
  const view = setup(),
    source = view.sources[0],
    original = structuredClone(view.stored())!
  act(() => view.result.current.setSourceSound(source, true))
  await tick()
  expect(view.result.current.revision).toBe(2)
  expect(view.result.current.dirty).toBe(false)
  expect(view.stored()).toEqual({
    ...original,
    sourceAudio: [
      { ...original.sourceAudio![0], retainOriginalAudio: true },
      original.sourceAudio![1],
    ],
  })
  act(() => view.result.current.setSourceSound(source, true))
  await tick()
  expect(view.writes).toHaveLength(1)
  act(() => view.result.current.dispatch({ type: 'undo' }))
  await tick()
  act(() => view.result.current.dispatch({ type: 'redo' }))
  await tick()
  expect(view.writes).toEqual(
    [true, false, true].map((enabled, i) => ({
      kind: 'sound',
      revision: i + 1,
      sourceId: 'a',
      enabled,
    })),
  )
  expect(view.result.current.revision).toBe(4)
})

it('serializes a toggle behind an in-flight plan save, retaining newer typing and an undo during sound IO', async () => {
  const view = setup(),
    source = view.sources[0]
  view.hold('plan')
  act(() =>
    view.result.current.change({ type: 'cut', id: 'cut-a', patch: { volumePermille: 800 } }),
  )
  await tick()
  act(() => view.result.current.setSourceSound(source, true))
  act(() =>
    view.result.current.change({
      type: 'text',
      id: 'caption-a',
      patch: { text: 'Exact owner copy' },
    }),
  )
  view.hold('sound')
  await act(async () => view.release())
  await tick()
  expect(view.writes.map((w) => [w.kind, w.revision])).toEqual([
    ['plan', 1],
    ['sound', 2],
  ])
  expect(view.result.current.draft.elements![0].text).toBe('Exact owner copy')
  // Undo the text and the sound while that sound write is already in flight.
  act(() => {
    view.result.current.dispatch({ type: 'undo' })
    view.result.current.dispatch({ type: 'undo' })
  })
  view.hold(undefined)
  await act(async () => view.release())
  await tick()
  expect(view.writes.map((w) => [w.kind, w.revision])).toEqual([
    ['plan', 1],
    ['sound', 2],
    ['sound', 3],
  ])
  expect(view.result.current.soundValue(source)).toBe(false)
  expect(view.result.current.draft.cuts[0].volumePermille).toBe(800)
  expect(view.result.current.dirty).toBe(false)
})

it('preserves intended sound and local copy across a revision conflict and explicit reapply', async () => {
  const view = setup(),
    source = view.sources[0]
  act(() => view.result.current.setSourceSound(source, true))
  act(() =>
    view.result.current.change({
      type: 'text',
      id: 'caption-a',
      patch: { text: 'Keep this draft' },
    }),
  )
  view.remote()
  await tick()
  expect(view.result.current.failure?.reason).toBe('CLIP_PLAN_CONFLICT')
  expect(view.result.current.soundValue(source)).toBe(false)
  expect(view.result.current.draft.elements![0].text).toBe('Keep this draft')
  act(() => view.result.current.reapply())
  await tick()
  await tick()
  expect(view.result.current.soundValue(source)).toBe(true)
  expect(view.result.current.dirty).toBe(false)
  expect(view.writes.map((w) => [w.kind, w.revision])).toEqual([
    ['sound', 1],
    ['sound', 2],
    ['plan', 3],
  ])
})

it('persists unused-source permission without adding it to the server cut snapshot', async () => {
  const view = setup(),
    source = view.sources[2]
  act(() => view.result.current.setSourceSound(source, true))
  await tick()
  expect(view.result.current.dirty).toBe(false)
  expect(view.result.current.soundValue(source)).toBe(true)
  expect(view.writes).toHaveLength(1)
  act(() =>
    view.result.current.change({ type: 'text', id: 'caption-a', patch: { text: 'Changed copy' } }),
  )
  await tick()
  expect(view.result.current.dirty).toBe(false)
  act(() => {
    view.result.current.dispatch({ type: 'undo' })
    view.result.current.dispatch({ type: 'undo' })
  })
  await tick()
  expect(view.result.current.soundValue(source)).toBe(false)
  expect(view.result.current.dirty).toBe(false)
})
