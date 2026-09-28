import { describe, expect, it } from 'vitest'
import { clipNarrationFixture, clipTimelineFixture } from '@/test/clip-editing'
import {
  applyTimelineEdit,
  captionStartCut,
  clipDraftKey,
  clipTimelineReducer,
  createClipTimeline,
  selectedTime,
  textInterval,
  timelineBarPx,
  timelineLabelFits,
  validateTimelinePlan,
} from './timeline'
import type { ClipEditPlan, ClipEditableText, ClipEditingState } from './edit-plan'

const text = (id: string, patch: Partial<ClipEditableText> = {}): ClipEditableText => ({
  instanceId: id,
  elementId: id,
  cutId: 'a',
  kind: 'fixed',
  role: 'caption',
  text: '오늘',
  rows: [],
  style: 'bold',
  position: 'bottom',
  align: 'center',
  basis: 'cut',
  pace: 'steady',
  accent: '',
  keyword: '',
  resolvedStartMs: 120,
  resolvedEndMs: 9880,
  groupId: '',
  itemId: '',
  ...patch,
})
function fixture(): ClipEditPlan {
  return {
    nativeComposition: true,
    durationMs: 19800,
    associations: [],
    cuts: ['a', 'b'].map((id, i) => ({
      id,
      sourceId: id,
      fingerprint: id,
      startMs: 1000,
      endMs: 11000,
      transitionMs: i ? 200 : 0,
      copies: [],
      volumePermille: 1000,
      playbackRatePermille: 1000,
    })),
    elements: [
      text('caption'),
      text('global', { basis: 'output-end', startMs: -1000, endMs: 0 }),
      text('price', {
        kind: 'ai',
        groupId: 'menu',
        itemId: 'rice',
        text: '밥 8,000원',
        evidence: [{ sourceId: 'a', fingerprint: 'a', startMs: 1000, endMs: 11000 }],
      }),
    ],
  }
}
const editing = (plan: ClipEditPlan): ClipEditingState => ({
  plan,
  sources: plan.cuts.map((c) => ({
    id: c.sourceId,
    fingerprint: c.fingerprint,
    filename: c.id,
    durationMs: 20000,
    width: 1920,
    height: 1080,
    allowedRatePermille: [500, 750, 1000, 1250, 1500, 2000],
  })),

  fadeMs: 200,
  maxCuts: 100,
  maxCopyRunes: 500,
  minDurationMs: 15000,
  maxDurationMs: 90000,
})

describe('timeline transactions', () => {
  it('keeps selection through reorder and playback; deleting selects a neighbour and preserves global text', () => {
    let state = createClipTimeline(fixture())
    state = clipTimelineReducer(state, {
      type: 'select',
      selection: { kind: 'text', id: 'caption' },
    })
    state = clipTimelineReducer(state, {
      type: 'edit',
      edit: { type: 'move', from: 0, to: 1 },
      at: 1,
    })
    expect(state.selection).toEqual({ kind: 'text', id: 'caption' })
    expect(selectedTime(state.plan, state.selection!)).toBe(10120)
    state = clipTimelineReducer(state, { type: 'seek', timeMs: 1000 })
    expect(state.selection?.id).toBe('caption')
    state = clipTimelineReducer(state, { type: 'edit', edit: { type: 'remove', id: 'a' }, at: 2 })
    expect(state.selection).toEqual({ kind: 'cut', id: 'b' })
    expect(state.plan.elements?.map((t) => t.instanceId)).toEqual(['global'])
    expect(textInterval(state.plan, state.plan.elements![0])).toMatchObject({
      startMs: 9000,
      endMs: 10000,
      valid: true,
    })
    state = clipTimelineReducer(state, { type: 'undo' })
    expect(state.plan.cuts.map((c) => c.id)).toEqual(['b', 'a'])
    expect(state.selection?.id).toBe('caption')
  })
  it('coalesces typing, bounds history, and keeps saved transactions undoable', () => {
    let state = createClipTimeline(fixture())
    for (let i = 0; i < 110; i++)
      state = clipTimelineReducer(state, {
        type: 'edit',
        edit: { type: 'text', id: 'caption', patch: { text: String(i) } },
        at: i * 1000,
        group: 'caption',
      })
    expect(state.past).toHaveLength(100)
    state = clipTimelineReducer(state, {
      type: 'edit',
      edit: { type: 'text', id: 'caption', patch: { text: '마지막' } },
      at: 109001,
      group: 'caption',
    })
    expect(state.past).toHaveLength(100)
    state = clipTimelineReducer(state, { type: 'adopt', plan: state.plan })
    state = clipTimelineReducer(state, { type: 'undo' })
    expect(state.plan.elements![0].text).toBe('108')
    expect(clipTimelineReducer(state, { type: 'redo' }).plan.elements![0].text).toBe('마지막')
  })
  it('leaves authored out-of-bounds times visible and native footage does not require legacy captions', () => {
    const initial = fixture()
    expect(validateTimelinePlan(initial, editing(initial)).valid).toBe(true)
    initial.elements![0].endMs = 9999
    initial.elements![0].startMs = 100
    const changed = applyTimelineEdit(initial, { type: 'cut', id: 'a', patch: { endMs: 10000 } })
    expect(changed.elements![0].endMs).toBe(9999)
    expect(validateTimelinePlan(changed, editing(initial)).elements[0].interval).toBe(true)
    expect(validateTimelinePlan(changed, editing(initial)).valid).toBe(false)
  })
  it('keeps item prices and original evidence unchanged when the owner links a different item', () => {
    const initial = fixture(),
      evidence = structuredClone(initial.elements![2].evidence)
    let state = createClipTimeline(initial)
    state = clipTimelineReducer(state, {
      type: 'edit',
      at: 1,
      edit: {
        type: 'associations',
        associations: [
          {
            groupId: 'menu',
            itemId: 'soup',
            sourceId: 'a',
            fingerprint: 'a',
            startMs: 1000,
            endMs: 11000,
          },
        ],
      },
    })
    expect(state.plan.elements![2]).toMatchObject({
      text: '밥 8,000원',
      staleEvidence: true,
      evidence,
    })
    expect(validateTimelinePlan(state.plan, editing(initial)).valid).toBe(false)
    expect(clipTimelineReducer(state, { type: 'undo' }).plan).toEqual(initial)
  })
  it('keeps manual phrase windows, reports overlap, and reflects phrase edits in the sentence', () => {
    const initial = fixture()
    const changed = applyTimelineEdit(initial, {
      type: 'text',
      id: 'caption',
      patch: {
        pace: 'rapid',
        phrases: [
          { text: '오늘은', startMs: 120, endMs: 420 },
          { text: '밥', startMs: 400, endMs: 900 },
        ],
      },
    })
    expect(changed.elements![0].text).toBe('오늘은 밥')
    expect(validateTimelinePlan(changed, editing(initial)).elements[0].phrases).toBe(true)
    expect(changed.elements![0].phrases![1].startMs).toBe(400)
  })
})

it('recomputes automatic placement at a new rate and preserves explicit phrase and caption milliseconds', () => {
  const initial = fixture()
  initial.elements![0] = text('caption', {
    startMs: 100,
    endMs: 6000,
    phrases: [{ text: '오늘', startMs: 5400, endMs: 6000 }],
    pace: 'rapid',
  })
  initial.elements!.push(text('auto', { effectiveStartMs: 120, effectiveEndMs: 9880 }))
  const changed = applyTimelineEdit(initial, {
    type: 'cut',
    id: 'a',
    patch: { playbackRatePermille: 2000 },
  })
  expect(changed.durationMs).toBe(14800)
  expect(changed.elements![0]).toMatchObject({
    startMs: 100,
    endMs: 6000,
    phrases: initial.elements![0].phrases,
  })
  expect(textInterval(changed, changed.elements!.at(-1)!)).toMatchObject({
    startMs: 120,
    endMs: 4880,
    valid: true,
  })
  expect(validateTimelinePlan(changed, editing(initial))).toMatchObject({
    valid: false,
    saveable: false,
    elements: [{ interval: true }, {}, {}, {}],
  })
  expect(changed.elements![2].evidence).toEqual(initial.elements![2].evidence)
})

it('adds footage with stable identity, then splits without copying left-side claims or sound authority', () => {
  const initial = fixture()
  initial.sourceAudio = [
    { sourceId: 'a', fingerprint: 'a', retainOriginalAudio: false },
    { sourceId: 'b', fingerprint: 'b', retainOriginalAudio: true },
  ]
  let state = createClipTimeline(initial)
  const id = 'owner-00000000-0000-4000-8000-000000000001'
  state = clipTimelineReducer(state, {
    type: 'edit',
    at: 1,
    edit: {
      type: 'addCut',
      id,
      originCutId: 'a',
      sourceId: 'a',
      fingerprint: 'a',
      startMs: 11000,
      endMs: 15000,
      focal: { x: 0.25, y: 0.75 },
    },
  })
  expect(state.plan.cuts.map((c) => c.id)).toEqual(['a', id, 'b'])
  expect(state.plan.cuts[1]).toMatchObject({
    playbackRatePermille: 1000,
    transitionMs: 0,
    volumePermille: 1000,
    copies: [],
    creation: { kind: 'add', originCutId: 'a' },
  })
  expect(state.plan.sourceAudio).toEqual(initial.sourceAudio)
  expect(state.selection).toEqual({ kind: 'cut', id })
  expect(state.timeMs).toBe(10000)
  state = clipTimelineReducer(state, { type: 'undo' })
  expect(state.plan.cuts).toEqual(initial.cuts)
  state = clipTimelineReducer(state, { type: 'redo' })
  expect(state.plan.cuts[1].id).toBe(id)
  const accepted = {
    ...state.plan,
    cuts: state.plan.cuts.map((c) => ({ ...c, creation: undefined })),
  }
  state = clipTimelineReducer(state, { type: 'adopt', plan: accepted })
  state = clipTimelineReducer(state, { type: 'undo' })
  state = clipTimelineReducer(state, { type: 'redo' })
  expect(state.plan.cuts[1].creation).toBeUndefined()
  state = clipTimelineReducer(state, { type: 'edit', at: 2, edit: { type: 'remove', id } })
  state = clipTimelineReducer(state, { type: 'adopt', plan: state.plan })
  state = clipTimelineReducer(state, { type: 'undo' })
  expect(state.plan.cuts[1]).toEqual(accepted.cuts[1])

  initial.cuts[0].playbackRatePermille = 500
  initial.cuts[0].volumePermille = 300
  initial.elements![0] = text('caption', { startMs: 120, endMs: 15000 })
  const split = applyTimelineEdit(initial, { type: 'splitCut', id: 'a', newId: id, sourceMs: 6000 })
  expect(split.cuts[0]).toMatchObject({ id: 'a', endMs: 6000, playbackRatePermille: 500 })
  expect(split.cuts[1]).toMatchObject({
    id,
    startMs: 6000,
    endMs: 11000,
    playbackRatePermille: 500,
    volumePermille: 300,
    transitionMs: 0,
    copies: [],
    creation: { kind: 'split', originCutId: 'a' },
  })
  expect(split.elements?.find((t) => t.instanceId === 'caption')).toMatchObject({
    cutId: 'a',
    endMs: 15000,
  })
  expect(split.elements?.some((t) => t.cutId === id)).toBe(false)
  expect(validateTimelinePlan(split, editing(initial)).saveable).toBe(false)
  expect(split.sourceAudio).toEqual(initial.sourceAudio)
})

it('keeps the source frame under the playhead through rate changes and identifies observation gaps', () => {
  const initial = fixture()
  let state = createClipTimeline(initial)
  state = clipTimelineReducer(state, { type: 'seek', timeMs: 4000 })
  state = clipTimelineReducer(state, {
    type: 'edit',
    at: 1,
    edit: { type: 'rate', id: 'a', ratePermille: 500 },
  })
  expect(state.timeMs).toBe(8000)
  state = clipTimelineReducer(state, { type: 'undo' })
  expect(state.timeMs).toBe(4000)
  const changed = applyTimelineEdit(initial, { type: 'cut', id: 'a', patch: { endMs: 12000 } })
  const observations = {
    status: 'available' as const,
    sources: [
      {
        source: editing(initial).sources[0],
        segments: [
          {
            startMs: 1000,
            endMs: 11000,
            event: '',
            action: '',
            motion: '',
            speech: '',
            quality: '',
            subjects: [],
            certainty: 'certain' as const,
            usability: 'usable' as const,
          },
        ],
      },
    ],
  }
  const checked = validateTimelinePlan(changed, editing(initial), observations)
  expect(checked).toMatchObject({
    saveable: false,
    cuts: [{ evidence: true }, { evidence: false }],
  })
  expect(changed.cuts[0].endMs).toBe(12000)
})

it('copies a newly used source permission from its retained lease without changing other sound settings', () => {
  const initial = fixture()
  initial.sourceAudio = [
    { sourceId: 'a', fingerprint: 'a', retainOriginalAudio: false },
    { sourceId: 'b', fingerprint: 'b', retainOriginalAudio: true },
  ]
  for (const retainedSound of [false, true]) {
    const added = applyTimelineEdit(initial, {
      type: 'addCut',
      id: 'owner-00000000-0000-4000-8000-000000000001',
      originCutId: 'a',
      sourceId: 'c',
      fingerprint: 'c',
      startMs: 0,
      endMs: 5000,
      focal: { x: 0.5, y: 0.5 },
      retainedSound,
    })
    expect(added.sourceAudio).toEqual([
      ...initial.sourceAudio,
      { sourceId: 'c', fingerprint: 'c', retainOriginalAudio: retainedSound },
    ])
    expect(initial.sourceAudio).toHaveLength(2)
    expect(added.cuts[1].volumePermille).toBe(1000)
  }
})

it('carries an owner placement through the draft queue and undo', () => {
  const fixture = clipTimelineFixture().plan
  let state = createClipTimeline(fixture)
  const id = fixture.elements![0].instanceId
  const before = clipDraftKey(state.plan)
  state = clipTimelineReducer(state, {
    type: 'edit',
    edit: { type: 'text', id, patch: { ownerPosition: { x: 200, y: 900 }, ownerStyle: 'film' } },
    at: 0,
  })
  const placed = state.plan.elements!.find((text) => text.instanceId === id)!
  expect(placed.ownerPosition).toEqual({ x: 200, y: 900 })
  expect(placed.ownerStyle).toBe('film')
  // A placement is a change the queue has to save, and one undo takes it back.
  expect(clipDraftKey(state.plan)).not.toBe(before)
  state = clipTimelineReducer(state, { type: 'undo' })
  expect(state.plan.elements!.find((text) => text.instanceId === id)!.ownerPosition).toBeUndefined()
  expect(clipDraftKey(state.plan)).toBe(before)
})

// CLIP-55, CLIP-191, CDS-100: a style, a size and a place are edits like any other. A new style
// that leaves the kept size outside its role makes the draft unsaveable on that caption — never
// clears the size — and undo steps back out of it; redo brings it back, still reported.
it('undoes and redoes a caption’s style and size, out of a size the new style cannot take', () => {
  const plan = clipTimelineFixture().plan
  const id = plan.elements![0].instanceId
  const state0 = createClipTimeline(plan)
  const edit = (state: typeof state0, patch: Partial<ClipEditableText>) =>
    clipTimelineReducer(state, { type: 'edit', edit: { type: 'text', id, patch }, at: 0 })
  const caption = (state: typeof state0) => state.plan.elements!.find((t) => t.instanceId === id)!
  const sized = edit(state0, { ownerSizePx: 64 })
  expect(validateTimelinePlan(sized.plan, editing(plan)).saveable).toBe(true)
  // keynote sets captions at 72–84: the kept 64 is reported, not reset.
  const restyled = edit(sized, { ownerStyle: 'keynote' })
  expect(caption(restyled)).toMatchObject({ ownerStyle: 'keynote', ownerSizePx: 64 })
  const refused = validateTimelinePlan(restyled.plan, editing(plan))
  expect(refused.saveable).toBe(false)
  expect(refused.elements.find((e) => e.id === id)?.size).toBe(true)
  const undone = clipTimelineReducer(restyled, { type: 'undo' })
  expect(caption(undone).ownerStyle).toBeUndefined()
  expect(caption(undone).ownerSizePx).toBe(64)
  expect(validateTimelinePlan(undone.plan, editing(plan)).saveable).toBe(true)
  const redone = clipTimelineReducer(undone, { type: 'redo' })
  expect(caption(redone).ownerStyle).toBe('keynote')
  expect(validateTimelinePlan(redone.plan, editing(plan)).saveable).toBe(false)
  // Correcting the size is what clears it.
  expect(validateTimelinePlan(edit(redone, { ownerSizePx: 72 }).plan, editing(plan)).saveable).toBe(
    true,
  )
})

// The size is checked against the style the caption is DRAWN in: a caption the owner gave no
// style takes its plan's, or the AI set's first where it names none.
it('bounds an owner size by the style the caption is drawn in', () => {
  const plan = clipTimelineFixture().plan
  const id = plan.elements![0].instanceId
  const with64 = (style: string) => ({
    ...plan,
    elements: plan.elements!.map((t) =>
      t.instanceId === id ? { ...t, style, ownerSizePx: 64 } : t,
    ),
  })
  const size = (candidate: ClipEditPlan, aiSet: string[]) =>
    validateTimelinePlan(candidate, editing(plan), undefined, aiSet).elements.find(
      (e) => e.id === id,
    )?.size
  expect(size(with64('keynote'), ['bold'])).toBe(true)
  expect(size(with64('auto'), ['keynote'])).toBe(true)
  expect(size(with64('auto'), ['bold'])).toBe(false)
  expect(size(with64('film'), ['keynote'])).toBe(false)
})

// CLIP-134, CLIP-143: a caption is placed over the frame of the cut its interval opens in —
// one crossing a cut boundary over the cut it starts in, several on one cut over that cut.
it('places a caption over the cut its interval starts in', () => {
  const plan = clipNarrationFixture().plan
  const caption = (id: string) => plan.elements!.find((t) => t.instanceId === id)!
  expect(captionStartCut(plan, caption('narration-1'))?.cut.id).toBe('cut-a')
  // 8–12 s crosses the cut boundary at 9.8 s: it opens over cut a.
  expect(captionStartCut(plan, caption('narration-2'))?.cut.id).toBe('cut-a')
  const late = { ...caption('narration-2'), instanceId: 'late', startMs: 10000, endMs: 12000 }
  expect(captionStartCut(plan, late)?.cut.id).toBe('cut-b')
})

describe('timeline label geometry', () => {
  // The strip the component draws for a 10 s plan at 80 px/s.
  const strip = 800
  it('measures a bar as its share of the strip', () => {
    expect(timelineBarPx(1000, 10000, strip)).toBe(80)
    expect(timelineBarPx(10000, 10000, strip)).toBe(strip)
    // A cut longer than the plan it sits in cannot draw past the strip.
    expect(timelineBarPx(20000, 10000, strip)).toBe(strip)
  })
  it('measures nothing from a length, a duration or a strip it cannot use', () => {
    const cases: Array<[number, number, number]> = [
      [0, 10000, strip],
      [-1, 10000, strip],
      [1000, 0, strip],
      [1000, Number.NaN, strip],
      [1000, 10000, 0],
    ]
    for (const [span, duration, px] of cases) {
      expect(timelineBarPx(span, duration, px)).toBe(0)
      expect(timelineLabelFits(span, duration, px)).toBe(false)
    }
  })
  it('admits a label only once the bar can hold one', () => {
    // 44 px is the floor: 550 ms of a 10 s plan is exactly 44 px.
    expect(timelineLabelFits(550, 10000, strip)).toBe(true)
    expect(timelineLabelFits(549, 10000, strip)).toBe(false)
    // Twenty cuts over 5 s: 20 px each, so none of them carries a label.
    expect(timelineLabelFits(250, 5000, 400)).toBe(false)
  })
})
