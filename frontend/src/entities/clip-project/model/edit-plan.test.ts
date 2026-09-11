import { expect, it } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { ClipEditingStateSchema } from '@/shared/api'
import { clipEditingFixture } from '@/test/clip-editing'
import { clipPlanToProto, toClipEditingState } from '../api/edit-plan'
import {
  copyChars,
  copyClipPlan,
  editClipPlan,
  minExposureMs,
  requiredClipSources,
  validateClipPlan,
  type ClipCaption,
  type ClipEditCut,
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
        // 메모 sits LEFT at the top or the bottom (CDS-24); anywhere else is
        // a placement the verifier refuses.
        anchor: 'bottom',
        align: 'left',
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

/** The design system's own rules, mirrored here for immediacy. The server's
 *  verifier stays the authority; these are what let a field say so first. */
it('mirrors the per-style limits, the exposure minimum and the per-clip guards', () => {
  const state = clipEditingFixture()
  const cut = (over: Partial<ClipEditCut> = {}, copy: Partial<ClipCaption> = {}) => ({
    ...state.plan.cuts[0]!,
    ...over,
    copy: { ...state.plan.cuts[0]!.copy, ...copy },
  })
  const check = (cuts: ClipEditCut[]) =>
    validateClipPlan({ ...state.plan, cuts, durationMs: 19800 }, state)

  // 깔끔하게 takes two lines of fourteen; fifteen on one line is past it.
  expect(check([cut({ id: 'a' }, { text: '가'.repeat(14) })]).cuts[0]!.text).toBe(false)
  expect(check([cut({ id: 'a' }, { text: '가'.repeat(15) })]).cuts[0]!.text).toBe(true)
  expect(check([cut({ id: 'a' }, { text: '가\n나\n다' })]).cuts[0]!.text).toBe(true)

  // CDS-41: 900 ms plus 90 ms a character, inside the copy's own window.
  expect(copyChars('여섯 글자다 !?')).toBe(5)
  expect(minExposureMs('여섯 글자다')).toBe(900 + 5 * 90)
  const tight = cut(
    { id: 'a', startMs: 0, endMs: 1589 },
    { text: '여섯 글자다', startMs: 0, endMs: 0 },
  )
  expect(check([tight]).cuts[0]!.exposure).toBe(true)
  expect(check([{ ...tight, endMs: 1590 }]).cuts[0]!.exposure).toBe(false)

  // 메모 sits LEFT at the top or the bottom (CDS-24).
  expect(
    check([cut({ id: 'a' }, { style: 'memo', anchor: 'top', align: 'left' })]).cuts[0]!.anchor,
  ).toBe(false)
  expect(
    check([cut({ id: 'a' }, { style: 'memo', anchor: 'top', align: 'center' })]).cuts[0]!.anchor,
  ).toBe(true)
  expect(
    check([cut({ id: 'a' }, { style: 'memo', anchor: 'lower_mid', align: 'left' })]).cuts[0]!
      .anchor,
  ).toBe(true)

  // CDS-38: one anchor step between consecutive cuts of the SAME style, and any
  // distance when the style changes.
  const stepped = [
    cut({ id: 'a' }, { anchor: 'bottom' }),
    cut({ id: 'b' }, { anchor: 'top', align: 'center' }),
  ]
  expect(check(stepped).cuts[1]!.anchor).toBe(true)
  stepped[1] = cut({ id: 'b' }, { anchor: 'lower_mid' })
  expect(check(stepped).cuts[1]!.anchor).toBe(false)
  stepped[1] = cut({ id: 'b' }, { style: 'memo', anchor: 'top', align: 'left' })
  expect(check(stepped).cuts[1]!.anchor).toBe(false)

  // At most two chips, from the reserved labels (CDS-30).
  expect(check([cut({ id: 'a', chips: ['위치', '가격'] })]).cuts[0]!.chips).toBe(false)
  expect(check([cut({ id: 'a', chips: ['위치', '가격', '메뉴'] })]).cuts[0]!.chips).toBe(true)
  expect(check([cut({ id: 'a', chips: ['주차'] })]).cuts[0]!.chips).toBe(true)

  // CDS-40's per-clip guards: 크게 강조 twice, and no style four in a row.
  const bolds = ['a', 'b', 'c'].map((id, i) =>
    cut({ id }, { style: 'bold', anchor: i === 0 ? 'upper_mid' : 'upper_mid', align: 'center' }),
  )
  expect(check(bolds).frequency).toBe(true)
  expect(check(bolds.slice(0, 2)).frequency).toBe(false)
  const run = ['a', 'b', 'c', 'd'].map((id) => cut({ id }, { style: 'clean' }))
  expect(check(run).frequency).toBe(true)
  expect(check(run.slice(0, 3)).frequency).toBe(false)

  // The accent word has to appear in the caption it accents.
  expect(check([cut({ id: 'a' }, { style: 'mark', keyword: '없음' })]).cuts[0]!.keyword).toBe(true)
  expect(
    check([cut({ id: 'a' }, { style: 'mark', text: '가격 9900원', keyword: '9900원' })]).cuts[0]!
      .keyword,
  ).toBe(false)
})
