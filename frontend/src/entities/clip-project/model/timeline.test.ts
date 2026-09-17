import { describe, expect, it } from 'vitest'
import { clipTimelineFixture } from '@/test/clip-editing'
import {
  applyTimelineEdit,
  clipDraftKey,
  clipTimelineReducer,
  createClipTimeline,
  selectedTime,
  textInterval,
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
    hook: '',
    associations: [],
    cuts: ['a', 'b'].map((id, i) => ({
      id,
      sourceId: id,
      fingerprint: id,
      startMs: 1000,
      endMs: 11000,
      transitionMs: i ? 200 : 0,
      copies: [],
      chips: [],
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
    chips: [],
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
    chips: [],
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
