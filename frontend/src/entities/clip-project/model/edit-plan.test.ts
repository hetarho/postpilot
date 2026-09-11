import { expect, it } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { ClipEditingStateSchema } from '@/shared/api'
import { clipEditingFixture } from '@/test/clip-editing'
import { clipPlanToProto, toClipEditingState } from '../api/edit-plan'
import {
  clipPlanDuration,
  copyChars,
  copyClipPlan,
  editClipPlan,
  groundedInAnswers,
  minExposureMs,
  requiredClipSources,
  validateClipPlan,
  withinHookLimits,
  type ClipCaption,
  type ClipEditCut,
  type ClipEditPlan,
} from './edit-plan'

it('immutably edits every field, reorders/deletes and recalculates transition overlap', () => {
  const state = clipEditingFixture(),
    snapshot = structuredClone(state)
  let plan = editClipPlan(state.plan, { type: 'move', from: 0, to: 1 })
  expect(plan.cuts.map((c) => c.id)).toEqual(['cut-b', 'cut-a'])
  // The fade rode cut-b, and cut-b is now first: a clip does not fade in from
  // nothing, so the reorder cleared it and the clip is its whole footage.
  expect(plan.cuts.map((c) => c.transitionMs)).toEqual([0, 0])
  expect(plan.durationMs).toBe(20000)
  plan = editClipPlan(plan, {
    type: 'cut',
    id: 'cut-b',
    patch: { startMs: 1000, endMs: 21000, volumePermille: 123 },
  })
  plan = editClipPlan(plan, {
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
  })
  expect(plan.durationMs).toBe(30000)
  // The owner fades into the cut that now follows, and only that boundary is
  // taken off the timeline (CDS-36).
  plan = editClipPlan(plan, { type: 'cut', id: 'cut-a', patch: { transitionMs: 200 } })
  expect(plan.durationMs).toBe(29800)
  expect(validateClipPlan(plan, state).valid).toBe(true)
  plan = editClipPlan(plan, { type: 'remove', id: 'cut-a' })
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
    // CDS-36 admits a cut, a 200 ms fade and a 300 ms fade-through-black.
    (p) => {
      p.cuts[1]!.transitionMs = 150
    },
  ],
  [
    // A clip does not fade in from nothing.
    (p) => {
      p.cuts[0]!.transitionMs = 200
    },
  ],
  [
    (p) => {
      p.cuts[0]!.copies[0]!.startMs = -1
    },
  ],
  [
    (p) => {
      p.cuts[0]!.copies[0]!.endMs = 10001
    },
  ],
  [
    (p) => {
      p.cuts[0]!.copies[0]!.text = '가'.repeat(501)
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
  state.plan.cuts[0]!.copies[0]!.style = 'bold'
  expect(validateClipPlan(state.plan, state).cuts[0]!.copies[0]!.style).toBe(true)
})

/** The design system's own rules, mirrored here for immediacy. The server's
 *  verifier stays the authority; these are what let a field say so first. */
it('mirrors the per-style limits, the exposure minimum and the per-clip guards', () => {
  const state = clipEditingFixture()
  const cut = (over: Partial<ClipEditCut> = {}, copy: Partial<ClipCaption> = {}) => ({
    ...state.plan.cuts[0]!,
    ...over,
    copies: [{ ...state.plan.cuts[0]!.copies[0]!, ...copy }],
  })
  const check = (cuts: ClipEditCut[]) =>
    validateClipPlan({ ...state.plan, cuts, durationMs: 19800 }, state)

  // 깔끔하게 takes two lines of fourteen; fifteen on one line is past it.
  expect(check([cut({ id: 'a' }, { text: '가'.repeat(14) })]).cuts[0]!.copies[0]!.text).toBe(false)
  expect(check([cut({ id: 'a' }, { text: '가'.repeat(15) })]).cuts[0]!.copies[0]!.text).toBe(true)
  expect(check([cut({ id: 'a' }, { text: '가\n나\n다' })]).cuts[0]!.copies[0]!.text).toBe(true)

  // CDS-41: 900 ms plus 90 ms a character, inside the copy's own window.
  expect(copyChars('여섯 글자다 !?')).toBe(5)
  expect(minExposureMs('여섯 글자다')).toBe(900 + 5 * 90)
  const tight = cut(
    { id: 'a', startMs: 0, endMs: 1589 },
    { text: '여섯 글자다', startMs: 0, endMs: 0 },
  )
  expect(check([tight]).cuts[0]!.copies[0]!.exposure).toBe(true)
  expect(check([{ ...tight, endMs: 1590 }]).cuts[0]!.copies[0]!.exposure).toBe(false)

  // 메모 sits LEFT at the top or the bottom (CDS-24).
  expect(
    check([cut({ id: 'a' }, { style: 'memo', anchor: 'top', align: 'left' })]).cuts[0]!.copies[0]!
      .anchor,
  ).toBe(false)
  expect(
    check([cut({ id: 'a' }, { style: 'memo', anchor: 'top', align: 'center' })]).cuts[0]!.copies[0]!
      .anchor,
  ).toBe(true)
  expect(
    check([cut({ id: 'a' }, { style: 'memo', anchor: 'lower_mid', align: 'left' })]).cuts[0]!
      .copies[0]!.anchor,
  ).toBe(true)

  // CDS-38: one anchor step between consecutive cuts of the SAME style, and any
  // distance when the style changes.
  const stepped = [
    cut({ id: 'a' }, { anchor: 'bottom' }),
    cut({ id: 'b' }, { anchor: 'top', align: 'center' }),
  ]
  expect(check(stepped).cuts[1]!.copies[0]!.anchor).toBe(true)
  stepped[1] = cut({ id: 'b' }, { anchor: 'lower_mid' })
  expect(check(stepped).cuts[1]!.copies[0]!.anchor).toBe(false)
  stepped[1] = cut({ id: 'b' }, { style: 'memo', anchor: 'top', align: 'left' })
  expect(check(stepped).cuts[1]!.copies[0]!.anchor).toBe(false)

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
  expect(
    check([cut({ id: 'a' }, { style: 'mark', keyword: '없음' })]).cuts[0]!.copies[0]!.keyword,
  ).toBe(true)
  expect(
    check([cut({ id: 'a' }, { style: 'mark', text: '가격 9900원', keyword: '9900원' })]).cuts[0]!
      .copies[0]!.keyword,
  ).toBe(false)
})

/** The hook card's own sentence (CDS-28, CDS-42). The field refuses what the
 *  compiler would drop, so the owner sees why rather than losing the card. */
it("holds the hook to two lines of nine, grounded in the owner's answers", () => {
  const state = clipEditingFixture(),
    answers = [
      { label: '상호', text: '해람 베이커리' },
      { label: '가격', text: '9,900원' },
    ]
  expect(withinHookLimits('아홉 글자까지만')).toBe(true)
  expect(copyChars('가'.repeat(18))).toBe(18)
  expect(withinHookLimits('가'.repeat(18))).toBe(true)
  expect(withinHookLimits('가'.repeat(19))).toBe(false)
  expect(withinHookLimits('한 줄\n두 줄\n세 줄')).toBe(false)

  // A number or a Latin name has to come from step ①; ordinary prose does not.
  expect(groundedInAnswers('', answers)).toBe(true)
  expect(groundedInAnswers('9900원 빵집', answers)).toBe(true)
  expect(groundedInAnswers('12000원 빵집', answers)).toBe(false)
  expect(groundedInAnswers('해람 Bakery', [{ label: '상호', text: '해람 Bakery' }])).toBe(true)
  expect(groundedInAnswers('해람 Bakery', answers)).toBe(false)
  expect(groundedInAnswers('역대급 빵집', answers)).toBe(false)
  expect(groundedInAnswers('빵집 🥐', answers)).toBe(false)

  // The whole plan carries it, and an invalid hook invalidates the plan.
  expect(validateClipPlan({ ...state.plan, hook: '아홉 글자까지만' }, state).hook).toBe(false)
  const long = validateClipPlan({ ...state.plan, hook: '가'.repeat(19) }, state)
  expect(long.hook).toBe(true)
  expect(long.valid).toBe(false)
  expect(editClipPlan(state.plan, { type: 'hook', hook: '갓 구운 빵' }).hook).toBe('갓 구운 빵')
  expect(
    toClipEditingState(
      create(ClipEditingStateSchema, {
        ...state,
        plan: clipPlanToProto({ ...state.plan, hook: '갓 구운 빵' }),
      }),
    ).plan.hook,
  ).toBe('갓 구운 빵')
})

// CDS-36: the duration is the footage less what each cut's own transition
// overlaps, and the same three transitions the renderer accepts.
it.each([
  [[0, 0, 0], 30000],
  [[0, 200, 200], 29600],
  [[0, 0, 200], 29800],
  [[0, 300, 0], 29700],
])('takes %s off the footage', (transitions, durationMs) => {
  const state = clipEditingFixture()
  const third = {
    ...state.plan.cuts[1]!,
    id: 'cut-c',
    copies: state.plan.cuts[1]!.copies.map((copy) => ({ ...copy })),
  }
  state.plan.cuts = [...state.plan.cuts, third].map((c, i) => ({
    ...c,
    transitionMs: transitions[i]!,
  }))
  state.plan.durationMs = clipPlanDuration(state.plan.cuts)
  expect(state.plan.durationMs).toBe(durationMs)
  expect(validateClipPlan(state.plan, state).timeline).toBe(false)
  // One millisecond either way is a timeline the renderer cannot make.
  expect(validateClipPlan({ ...state.plan, durationMs: durationMs + 1 }, state).timeline).toBe(true)
})
