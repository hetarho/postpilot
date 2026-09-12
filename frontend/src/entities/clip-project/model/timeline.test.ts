import { describe, expect, it } from 'vitest'
import {
  applyTimelineEdit,
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
  style: 'clean',
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
  })),
  copyStyles: ['clean'],
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
