import { expect, it } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { ClipEditingStateSchema } from '@/shared/api'
import { clipEditingFixture } from '@/test/clip-editing'
import { clipPlanToProto, toClipEditingState } from '../api/edit-plan'
import {
  clipPlanDuration,
  cutOutputMs,
  ownerCutId,
  sourceOverlaps,
  transformedDurationMs,
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
      style: 'bold',
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

  state.plan.cuts[0]!.copies[0]!.style = 'unknown' as ClipCaption['style']
  expect(validateClipPlan(state.plan, state).cuts[0]!.copies[0]!.style).toBe(true)
})

/** The design system's own rules, mirrored here for immediacy. The server's
 *  verifier stays the authority; these are what let a field say so first. */
it('checks caption limits and exposure without retired frequency guards', () => {
  const state = clipEditingFixture()
  const cut = (over: Partial<ClipEditCut> = {}, copy: Partial<ClipCaption> = {}) => ({
    ...state.plan.cuts[0]!,
    ...over,
    copies: [{ ...state.plan.cuts[0]!.copies[0]!, ...copy }],
  })
  const check = (cuts: ClipEditCut[]) =>
    validateClipPlan({ ...state.plan, cuts, durationMs: 19800 }, state)

  // 깔끔하게 takes two lines of fourteen; fifteen on one line is past it.
  expect(check([cut({ id: 'a' }, { text: '가'.repeat(11) })]).cuts[0]!.copies[0]!.text).toBe(false)
  expect(check([cut({ id: 'a' }, { text: '가'.repeat(12) })]).cuts[0]!.copies[0]!.text).toBe(true)
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

  // CDS-38: one anchor step between consecutive cuts of the SAME style, and any
  // distance when the style changes.
  const stepped = [
    cut({ id: 'a' }, { anchor: 'bottom' }),
    cut({ id: 'b' }, { anchor: 'top', align: 'center' }),
  ]
  expect(check(stepped).cuts[1]!.copies[0]!.anchor).toBe(true)
  stepped[1] = cut({ id: 'b' }, { anchor: 'lower_mid' })
  expect(check(stepped).cuts[1]!.copies[0]!.anchor).toBe(false)
  // At most two chips, from the reserved labels (CDS-30).
  expect(check([cut({ id: 'a', chips: ['위치', '가격'] })]).cuts[0]!.chips).toBe(false)
  expect(check([cut({ id: 'a', chips: ['위치', '가격', '메뉴'] })]).cuts[0]!.chips).toBe(true)
  expect(check([cut({ id: 'a', chips: ['주차'] })]).cuts[0]!.chips).toBe(true)

  // CDS-40's per-clip guards: 크게 강조 twice, and no style four in a row.
  const bolds = ['a', 'b', 'c'].map((id, i) =>
    cut({ id }, { style: 'bold', anchor: i === 0 ? 'upper_mid' : 'upper_mid', align: 'center' }),
  )
  expect(check(bolds).frequency).toBe(false)
  expect(check(bolds.slice(0, 2)).frequency).toBe(false)
  const run = ['a', 'b', 'c', 'd'].map((id) => cut({ id }, { style: 'bold' }))
  expect(check(run).frequency).toBe(false)
  expect(check(run.slice(0, 3)).frequency).toBe(false)

  // The accent word has to appear in the caption it accents.
  expect(
    check([cut({ id: 'a' }, { style: 'bold', keyword: '없음' })]).cuts[0]!.copies[0]!.keyword,
  ).toBe(true)
  expect(
    check([cut({ id: 'a' }, { style: 'bold', text: '가격 9900원', keyword: '9900원' })]).cuts[0]!
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

it('measures a rate-changed cut on the transformed output timeline', () => {
  const state = clipEditingFixture()
  // 10 s of footage at 2x is 5 s of output; the source range is untouched.
  const plan = editClipPlan(state.plan, {
    type: 'cut',
    id: 'cut-a',
    patch: { playbackRatePermille: 2000 },
  })
  expect(plan.cuts[0]!.startMs).toBe(0)
  expect(plan.cuts[0]!.endMs).toBe(10000)
  expect(cutOutputMs(plan.cuts[0]!)).toBe(5000)
  // The clip is now 5 s + 10 s less cut-b's own 200 ms fade.
  expect(plan.durationMs).toBe(14800)
  expect(clipPlanDuration(plan.cuts)).toBe(14800)
  // 0.75x of 10 s is 13333 ms, to the nearest millisecond and no float drift.
  expect(transformedDurationMs(10000, 750)).toBe(13333)
  expect(transformedDurationMs(3, 2000)).toBe(2)
  // Not a rate at all: zero, an unsupported value and an empty span.
  for (const [span, rate] of [
    [10000, 0],
    [10000, 900],
    [0, 1000],
    [-1, 1000],
  ])
    expect(transformedDurationMs(span!, rate!)).toBe(0)
})

it('refuses an unsupported rate, an inadmissible slow rate and a new overlap', () => {
  const state = clipEditingFixture()
  // 30 fps footage cannot reach the output at 0.75x, so the server never offers it.
  state.sources = state.sources.map((s) => ({ ...s, allowedRatePermille: [1000, 1250, 2000] }))
  const slow = editClipPlan(state.plan, {
    type: 'cut',
    id: 'cut-a',
    patch: { playbackRatePermille: 750 },
  })
  expect(validateClipPlan(slow, state).cuts[0]!.rate).toBe(true)
  // The rate is named, never replaced by 1x.
  expect(slow.cuts[0]!.playbackRatePermille).toBe(750)
  const bad = editClipPlan(state.plan, {
    type: 'cut',
    id: 'cut-a',
    patch: { playbackRatePermille: 900 },
  })
  expect(validateClipPlan(bad, state).cuts[0]!.rate).toBe(true)
  const fine = editClipPlan(state.plan, {
    type: 'cut',
    id: 'cut-a',
    patch: { playbackRatePermille: 1250 },
  })
  expect(validateClipPlan(fine, state).cuts[0]!.rate).toBe(false)

  // Two cuts of one source: touching endpoints are adjacent, sharing is not.
  const shared = copyClipPlan(state.plan)
  shared.cuts[1] = {
    ...shared.cuts[1]!,
    sourceId: 'a',
    fingerprint: shared.cuts[0]!.fingerprint,
    startMs: 10000,
    endMs: 20000,
  }
  expect(sourceOverlaps(shared.cuts).size).toBe(0)
  shared.cuts[1] = { ...shared.cuts[1]!, startMs: 5000, endMs: 15000 }
  expect([...sourceOverlaps(shared.cuts).values()]).toEqual([5000])
  const report = validateClipPlan(shared, state)
  expect(report.cuts[0]!.overlap).toBe(true)
  expect(report.valid).toBe(false)
})

it('carries the rate and the owner audio snapshot across the wire unchanged', () => {
  const state = clipEditingFixture()
  const wire = create(ClipEditingStateSchema, {
    plan: {
      ...clipPlanToProto(state.plan),
      sourceAudio: {
        values: [{ sourceId: 'a', fingerprint: 'a'.repeat(64), retainOriginalAudio: true }],
      },
    },
    sources: state.sources,
  })
  const back = toClipEditingState(wire)
  expect(back.plan.cuts.map((c) => c.playbackRatePermille)).toEqual([1000, 1000])
  expect(back.plan.sourceAudio).toEqual([
    { sourceId: 'a', fingerprint: 'a'.repeat(64), retainOriginalAudio: true },
  ])
  expect(back.sources[0]!.allowedRatePermille).toEqual([500, 750, 1000, 1250, 1500, 2000])
  // A server that predates rates sends neither: 1x, and the fast rates only.
  const legacy = create(ClipEditingStateSchema, {
    plan: {
      ...clipPlanToProto(state.plan),
      cuts: clipPlanToProto(state.plan).cuts.map((c) => ({
        ...c,
        playbackRatePermille: undefined,
      })),
    },
    sources: state.sources.map((s) => ({ ...s, allowedRatePermille: [] })),
  })
  const old = toClipEditingState(legacy)
  expect(old.plan.cuts.map((c) => c.playbackRatePermille)).toEqual([1000, 1000])
  expect(old.sources[0]!.allowedRatePermille).toEqual([1000, 1250, 1500, 2000])
})

it('sends creation provenance only for an id the saved plan does not contain', () => {
  const state = clipEditingFixture()
  const id = ownerCutId()
  expect(id).toMatch(/^owner-[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/u)
  const plan: ClipEditPlan = {
    ...state.plan,
    cuts: [
      ...state.plan.cuts,
      {
        ...state.plan.cuts[0]!,
        id,
        startMs: 20000,
        endMs: 30000,
        creation: { kind: 'add', originCutId: 'cut-a' },
      },
    ],
  }
  const wire = clipPlanToProto(plan)
  expect(wire.cuts[2]!.creation).toEqual({ kind: 'add', originCutId: 'cut-a' })
  // Every cut the server already approved carries none.
  expect(wire.cuts[0]!.creation).toBeUndefined()
  expect(wire.cuts[1]!.creation).toBeUndefined()
  // And a saved plan read back never carries it, so a resave is a correction.
  const back = toClipEditingState(
    create(ClipEditingStateSchema, {
      plan: clipPlanToProto(state.plan),
      sources: state.sources,
    }),
  )
  expect(back.plan.cuts.every((c) => c.creation === undefined)).toBe(true)
})
