import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { Code, createRouterTransport } from '@connectrpc/connect'
import {
  ClipGenerationService,
  ClipPlanService,
  ClipProjectSchema,
  ClipEditingStateSchema,
} from '@/shared/api'
import { clipPlanToProto, toClipEditingState } from '@/entities/clip-plan'
import * as clipPlan from '@/entities/clip-plan'
import { toClipProject, type ClipProject } from '@/entities/clip-project'
import { clipEditingFixture, clipTimelineFixture } from '@/test/clip-editing'
import { clipObservationsFixture } from '@/test/clip-observations'
import { connectAppError } from '@/test/app-error'
import { createTestQueryClient, withProviders } from '@/test/session'
import { CLIP_TIMELINE, clipCaptionSizes } from '@/entities/clip-design'
import { useClipCorrection } from './useClipCorrection'

function setup(editing = clipEditingFixture()) {
  const identity = vi.fn(() => 'owner-00000000-0000-4000-8000-000000000001')
  const creationRequests: string[][] = []
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
    router.rpc(ClipGenerationService.method.getClipProject, () => ({ project: wire() }))
    router.rpc(ClipPlanService.method.saveClipEditPlan, async (req) => {
      writes.push(req.expectedRevision)
      creationRequests.push(req.plan?.cuts.filter((c) => c.creation).map((c) => c.id) ?? [])
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
    ({ project }: { project: ClipProject }) => useClipCorrection('alice', project, identity),
    { wrapper: withProviders(transport, createTestQueryClient()), initialProps: { project } },
  )
  return {
    ...view,
    writes,
    identity,
    creationRequests,
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
afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})
const tick = async (ms: number = CLIP_TIMELINE.autosaveMs) => {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms)
  })
}

it('reuses the draft key and validation across 300 seeks, selection and unrelated rerenders', () => {
  const key = vi.spyOn(clipPlan, 'clipDraftKey')
  const validate = vi.spyOn(clipPlan, 'validateTimelinePlan')
  const view = setup(clipTimelineFixture())
  const { draft, validation } = view.result.current
  const keyCalls = key.mock.calls.length
  const validationCalls = validate.mock.calls.length

  for (let timeMs = 0; timeMs < 300; timeMs++)
    act(() => view.result.current.dispatch({ type: 'seek', timeMs }))
  expect(view.result.current.timeline.timeMs).toBe(299)
  act(() =>
    view.result.current.dispatch({ type: 'select', selection: { kind: 'cut', id: 'cut-b' } }),
  )
  view.rerender({ project: { ...view.project, title: 'Only the project title changed' } })

  expect(view.result.current.draft).toBe(draft)
  expect({ key: key.mock.calls.length, validation: validate.mock.calls.length }).toEqual({
    key: keyCalls,
    validation: validationCalls,
  })
  expect(view.result.current.validation).toBe(validation)
  expect(view.result.current.dirty).toBe(false)
  expect(view.writes).toEqual([])
})

it('refreshes the key on edits and undo and validation on changed or absent editing contracts', () => {
  const directValidation = clipPlan.validateTimelinePlan
  const validate = vi.spyOn(clipPlan, 'validateTimelinePlan')
  const view = setup(clipTimelineFixture())
  const editing = view.project.editing!
  const original = view.result.current.validation

  act(() =>
    view.result.current.change({ type: 'cut', id: 'cut-a', patch: { volumePermille: 800 } }),
  )
  expect(view.result.current.dirty).toBe(true)
  expect(view.result.current.validation).not.toBe(original)
  expect(view.result.current.validation).toEqual(
    directValidation(view.result.current.draft, editing),
  )
  act(() => view.result.current.dispatch({ type: 'undo' }))
  expect(view.result.current.dirty).toBe(false)
  expect(view.result.current.validation?.saveable).toBe(true)

  const draft = view.result.current.draft
  const restricted = { ...editing, maxCuts: 1 }
  view.rerender({ project: { ...view.project, editing: restricted } })
  expect(view.result.current.draft).toBe(draft)
  expect(view.result.current.validation).toEqual(directValidation(draft, restricted))
  expect(view.result.current.validation?.saveable).toBe(false)
  expect(view.result.current.dirty).toBe(false)

  const beforeAbsent = validate.mock.calls.length
  view.rerender({ project: { ...view.project, editing: undefined } })
  expect(view.result.current.validation).toBeUndefined()
  expect(validate).toHaveBeenCalledTimes(beforeAbsent)
  view.rerender({ project: view.project })
  expect(validate).toHaveBeenCalledTimes(beforeAbsent + 1)
  expect(view.result.current.validation).toEqual(directValidation(draft, editing))
  expect(view.result.current.validation?.saveable).toBe(true)
})

it('revalidates unchanged authored cuts when the server observations change', () => {
  const directValidation = clipPlan.validateTimelinePlan
  const view = setup(clipTimelineFixture())
  act(() => view.result.current.change({ type: 'cut', id: 'cut-a', patch: { startMs: 3500 } }))
  expect(view.result.current.validation?.saveable).toBe(true)
  const draft = view.result.current.draft
  const editing = view.project.editing!
  const unavailableEvidence = { status: 'available' as const, sources: [] }

  view.rerender({ project: { ...view.project, observations: unavailableEvidence } })
  expect(view.result.current.draft).toBe(draft)
  expect(view.result.current.validation).toEqual(
    directValidation(draft, editing, unavailableEvidence),
  )
  expect(view.result.current.validation?.cuts[0]).toMatchObject({ evidence: true })
  expect(view.result.current.validation?.saveable).toBe(false)

  const observations = clipObservationsFixture()
  view.rerender({ project: { ...view.project, observations } })
  expect(view.result.current.validation).toEqual(directValidation(draft, editing, observations))
  expect(view.result.current.validation?.cuts[0]).toMatchObject({ evidence: false })
  expect(view.result.current.validation?.saveable).toBe(true)
  expect(view.result.current.dirty).toBe(true)
})

it('revalidates the retained caption size when the project AI style set changes', () => {
  const directValidation = clipPlan.validateTimelinePlan
  const editing = clipTimelineFixture()
  editing.plan.elements![0] = {
    ...editing.plan.elements![0],
    style: 'auto',
    ownerSizePx: clipCaptionSizes('keynote').max,
  }
  const view = setup(editing)
  const draft = view.result.current.draft
  expect(view.result.current.validation?.elements[0].size).toBe(true)
  expect(view.result.current.validation?.saveable).toBe(false)

  view.rerender({ project: { ...view.project, allowedCaptionStyles: ['keynote'] } })
  expect(view.result.current.draft).toBe(draft)
  expect(view.result.current.validation).toEqual(
    directValidation(draft, view.project.editing!, view.project.observations, ['keynote']),
  )
  expect(view.result.current.validation?.elements[0].size).toBe(false)
  expect(view.result.current.validation?.saveable).toBe(true)
  expect(view.result.current.dirty).toBe(false)

  view.rerender({ project: view.project })
  expect(view.result.current.validation?.elements[0].size).toBe(true)
  expect(view.result.current.validation?.saveable).toBe(false)
})

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

it('flushes edits typed during an existing save without relying on another render or debounce', async () => {
  const view = setup()
  view.setHold(true)
  act(() =>
    view.result.current.change({ type: 'cut', id: 'cut-a', patch: { volumePermille: 800 } }),
  )
  await tick()
  act(() =>
    view.result.current.change({ type: 'cut', id: 'cut-a', patch: { volumePermille: 600 } }),
  )
  let revision: number | undefined
  const flushed = view.result.current.flush().then((r) => {
    revision = r
  })
  view.setHold(false)
  await act(async () => {
    view.release()
    await flushed
  })
  expect(view.writes).toEqual([1, 2])
  expect(revision).toBe(3)
  expect(view.result.current.draft.cuts[0].volumePermille).toBe(600)
  expect(view.result.current.dirty).toBe(false)
})

it('rejects a failing flush and preserves the local correction', async () => {
  const view = setup()
  view.setFail(true)
  act(() =>
    view.result.current.change({ type: 'cut', id: 'cut-a', patch: { volumePermille: 500 } }),
  )
  await act(async () => {
    await expect(view.result.current.flush()).rejects.toThrow()
  })
  expect(view.result.current.dirty).toBe(true)
  expect(view.writes).toEqual([1])
})

it('creates one injected UUID and strips provenance from saved undo/redo at new revisions', async () => {
  const view = setup(clipTimelineFixture())
  const source = view.project.editing!.sources[0]
  await act(async () =>
    view.result.current.addCut({
      source,
      startMs: 12000,
      endMs: 15000,
      segment: {
        startMs: 12000,
        endMs: 15000,
        event: '',
        action: '',
        motion: '',
        speech: '',
        subjects: [],
        quality: '',
        certainty: 'certain',
        usability: 'usable',
        focal: { x: 0.2, y: 0.7 },
      },
    }),
  )
  const id = view.identity.mock.results[0].value
  expect(view.identity).toHaveBeenCalledTimes(1)
  expect(view.result.current.draft.cuts[1]).toMatchObject({ id, creation: { kind: 'add' } })
  await tick()
  expect(view.creationRequests).toEqual([[id]])
  expect(view.result.current.draft.cuts[1].creation).toBeUndefined()
  act(() => view.result.current.dispatch({ type: 'undo' }))
  await tick()
  act(() => view.result.current.dispatch({ type: 'redo' }))
  await tick()
  expect(view.writes).toEqual([1, 2, 3])
  expect(view.creationRequests).toEqual([[id], [], []])
  expect(view.result.current.revision).toBe(4)
  expect(view.result.current.draft.cuts[1].id).toBe(id)
  expect(view.identity).toHaveBeenCalledTimes(1)
})

it('keeps a split and its exact left caption available when the authored window no longer fits', async () => {
  const view = setup(clipTimelineFixture())
  await act(async () => view.result.current.splitCut('cut-a', 2000))
  await tick(5000)
  expect(view.result.current.draft.cuts).toHaveLength(3)
  expect(view.result.current.draft.elements![0]).toMatchObject({
    cutId: 'cut-a',
    startMs: 120,
    endMs: 3880,
  })
  expect(view.result.current.validation?.saveable).toBe(false)
  expect(view.writes).toEqual([])
})

it('acknowledges creation while another cut is edited during its save', async () => {
  const view = setup(clipTimelineFixture())
  const source = view.project.editing!.sources[0]
  await act(async () =>
    view.result.current.addCut({
      source,
      startMs: 12000,
      endMs: 15000,
      segment: {
        startMs: 12000,
        endMs: 15000,
        event: '',
        action: '',
        motion: '',
        speech: '',
        subjects: [],
        quality: '',
        certainty: 'certain',
        usability: 'usable',
        focal: { x: 0.2, y: 0.7 },
      },
    }),
  )
  view.setHold(true)
  await tick()
  act(() =>
    view.result.current.change({ type: 'cut', id: 'cut-a', patch: { volumePermille: 300 } }),
  )
  view.setHold(false)
  await act(async () => view.release())
  await tick()
  expect(view.creationRequests).toEqual([[view.identity.mock.results[0].value], []])
  expect(view.result.current.draft.cuts[0].volumePermille).toBe(300)
  expect(view.result.current.revision).toBe(3)
  expect(view.result.current.dirty).toBe(false)
})

it('moves a cut by a place as one undoable edit and refuses a move past either end', () => {
  const view = setup()
  const order = () => view.result.current.draft.cuts.map((c) => c.id)
  expect(order()).toEqual(['cut-a', 'cut-b'])
  let moved = true
  act(() => {
    moved = view.result.current.moveCut('cut-a', -1)
  })
  expect(moved).toBe(false)
  act(() => {
    moved = view.result.current.moveCut('cut-a', 1)
  })
  expect(moved).toBe(true)
  expect(order()).toEqual(['cut-b', 'cut-a'])
  act(() => view.result.current.dispatch({ type: 'undo' }))
  expect(order()).toEqual(['cut-a', 'cut-b'])
})
