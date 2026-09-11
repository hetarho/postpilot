import { expect, it } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { ClipEditingStateSchema } from '@/shared/api'
import { clipEditingFixture } from '@/test/clip-editing'
import { clipPlanToProto, toClipEditingState } from '../api/edit-plan'
import {
  copyClipPlan,
  editClipPlan,
  requiredClipSources,
  validateClipPlan,
  type ClipEditPlan,
} from './edit-plan'

it('immutably edits every field, reorders/deletes and recalculates fade overlap', () => {
  const state = clipEditingFixture(),
    snapshot = structuredClone(state)
  let plan = editClipPlan(state.plan, { type: 'move', from: 0, to: 1 }, state.fadeMs)
  expect(plan.cuts.map((c) => c.id)).toEqual(['cut-b', 'cut-a'])
  plan = editClipPlan(
    plan,
    { type: 'cut', id: 'cut-b', patch: { startMs: 1000, endMs: 21000, volumePermille: 123 } },
    state.fadeMs,
  )
  plan = editClipPlan(
    plan,
    {
      type: 'copy',
      id: 'cut-b',
      patch: {
        text: '정확한 <문구>',
        startMs: 200,
        endMs: 3000,
        anchor: 'lower_mid',
        style: 'memo',
        accent: 'teal',
      },
    },
    state.fadeMs,
  )
  expect(plan.durationMs).toBe(29800)
  expect(validateClipPlan(plan, state).valid).toBe(true)
  plan = editClipPlan(plan, { type: 'remove', id: 'cut-a' }, state.fadeMs)
  expect(plan.durationMs).toBe(20000)
  expect(requiredClipSources(plan, state.sources).map((s) => s.id)).toEqual(['b'])
  expect(state).toEqual(snapshot)
  expect(
    toClipEditingState(create(ClipEditingStateSchema, { ...state, plan: clipPlanToProto(plan) }))
      .plan,
  ).toEqual(plan)
})
it.each<[(p: ClipEditPlan) => void]>([
  [
    (p) => {
      p.cuts = []
    },
  ],
  [
    (p) => {
      p.cuts[0]!.startMs = -1
    },
  ],
  [
    (p) => {
      p.cuts[0]!.endMs = 40001
    },
  ],
  [
    (p) => {
      p.cuts[0]!.startMs = 10000
    },
  ],
  [
    (p) => {
      p.cuts[0]!.endMs = 400
    },
  ],
  [
    (p) => {
      p.cuts[0]!.copy.startMs = -1
    },
  ],
  [
    (p) => {
      p.cuts[0]!.copy.endMs = 10001
    },
  ],
  [
    (p) => {
      p.cuts[0]!.copy.text = '가'.repeat(501)
    },
  ],
  [
    (p) => {
      p.cuts[0]!.volumePermille = -1
    },
  ],
  [
    (p) => {
      p.cuts[0]!.volumePermille = 1001
    },
  ],
  [
    (p) => {
      p.cuts[0]!.volumePermille = 1.1
    },
  ],
  [
    (p) => {
      p.cuts[0]!.startMs = NaN
    },
  ],
  [
    (p) => {
      p.durationMs = 14999
    },
  ],
  [
    (p) => {
      p.durationMs = 90001
    },
  ],
  [
    (p) => {
      p.cuts[0]!.fingerprint = 'forged'
    },
  ],
  [
    (p) => {
      p.cuts[1]!.id = p.cuts[0]!.id
    },
  ],
])('rejects an invalid plan without changing its baseline (%#)', (mutate) => {
  const state = clipEditingFixture(),
    draft = copyClipPlan(state.plan)
  mutate(draft)
  expect(validateClipPlan(draft, state).valid).toBe(false)
  expect(validateClipPlan(state.plan, state).valid).toBe(true)
})
it('accepts muted/full source audio and refuses a non-approved style', () => {
  const state = clipEditingFixture()
  state.plan.cuts[0]!.volumePermille = 0
  expect(validateClipPlan(state.plan, state).valid).toBe(true)
  state.copyStyles = ['clean', 'memo']
  state.plan.cuts[0]!.copy.style = 'bold'
  expect(validateClipPlan(state.plan, state).cuts[0]!.style).toBe(true)
})
