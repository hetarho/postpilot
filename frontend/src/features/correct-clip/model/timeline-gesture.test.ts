import { expect, it } from 'vitest'
import { clipNarrationFixture } from '@/test/clip-editing'
import {
  applyTimelineEdit,
  clipTimelineReducer,
  createClipTimeline,
  timelineCuts,
} from '@/entities/clip-plan'
import { cutGestureEdit, splitAtOutput, timelinePoint } from './timeline-gesture'

it('maps scrolled output geometry and clamps narrow/background boundaries to the shared clock', () => {
  expect(timelinePoint(110, -90, 400, 20000)).toBe(10000)
  expect(timelinePoint(-200, -90, 400, 20000)).toBe(0)
  expect(timelinePoint(900, -90, 400, 20000)).toBe(20000)
  expect(timelinePoint(20, 0, 0, 20000)).toBe(0)
})
it('trims in transformed source time, clamps observed bounds and keeps owner caption and speech placements', () => {
  const plan = clipNarrationFixture().plan
  plan.cuts[0].playbackRatePermille = 2000
  plan.narration = {
    enabled: true,
    confirmedVoiceId: 'v',
    bindingDigest: 'b',
    volumePermille: 1000,
    segments: [
      { id: 's', text: '안녕', textRevision: 1, inputHash: 'h', startMs: 9000, endMs: 11000 },
    ],
  }
  const edit = cutGestureEdit(plan, 'cut-a', 'end', -1000, 0, { startMs: 0, endMs: 10000 })!
  expect(edit).toEqual({ type: 'cut', id: 'cut-a', patch: { endMs: 8000 } })
  const after = applyTimelineEdit(plan, edit)
  expect(after.elements).toEqual(plan.elements)
  expect(after.narration).toEqual(plan.narration)
  const min = cutGestureEdit(plan, 'cut-a', 'start', 100000, 0, { startMs: 0, endMs: 10000 })!
  expect(min).toMatchObject({ patch: { startMs: 9198 } })
})
it('splits a single source at non-1x output time and refuses overlap, edges and undersized halves', () => {
  const plan = clipNarrationFixture().plan
  plan.cuts[0].playbackRatePermille = 2000
  expect(splitAtOutput(plan, 'cut-a', 1000, 'new')).toMatchObject({ sourceMs: 2000 })
  expect(splitAtOutput(plan, 'cut-a', 0, 'new')).toBeUndefined()
  expect(splitAtOutput(plan, 'cut-a', 4950, 'new')).toBeUndefined()
  expect(splitAtOutput(plan, 'cut-a', 100, 'new')).toBeUndefined()
  const next = applyTimelineEdit(plan, splitAtOutput(plan, 'cut-a', 1000, 'new')!)
  expect(next.cuts.slice(0, 2).map((c) => [c.startMs, c.endMs, c.sourceId])).toEqual([
    [0, 2000, 'a'],
    [2000, 10000, 'a'],
  ])
})
it('commits a reorder as one undo transaction with stable selection and preserved transition semantics', () => {
  const plan = clipNarrationFixture().plan
  let state = createClipTimeline(plan)
  state = clipTimelineReducer(state, { type: 'select', selection: { kind: 'cut', id: 'cut-a' } })
  const edit = cutGestureEdit(plan, 'cut-a', 'move', 1000, 19000, { startMs: 0, endMs: 10000 })!
  state = clipTimelineReducer(state, { type: 'edit', edit, at: 1 })
  expect(state.past).toHaveLength(1)
  expect(state.selection?.id).toBe('cut-a')
  expect(timelineCuts(state.plan)[0].cut.transitionMs).toBe(0)
  expect(state.plan.elements).toEqual(plan.elements)
  state = clipTimelineReducer(state, { type: 'undo' })
  expect(state.plan).toEqual(plan)
})
