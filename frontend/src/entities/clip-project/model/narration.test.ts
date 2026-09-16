import { expect, it } from 'vitest'
import { clipNarrationFixture } from '@/test/clip-editing'
import { applyTimelineEdit, clipTextTracks, narrationSlot, nativeTextErrors } from './timeline'
import { clipNoticeKey } from './notices'

it('lays the narration in one lane whatever cut lies beneath it', () => {
  const plan = clipNarrationFixture().plan
  const [captions, ...rest] = clipTextTracks(plan)
  expect(captions.map((bar) => bar.id)).toEqual(['narration-1', 'narration-2'])
  // The second caption crosses the cut boundary at 9.8 s and is still ONE bar.
  expect(captions[1]).toMatchObject({ startMs: 8000, endMs: 12000 })
  // The template's fixed region keeps its own lane below.
  expect(rest.flat().map((bar) => bar.id)).toEqual(['global'])
})

it('adds a caption at the playhead in free room, and none where the narration already speaks', () => {
  const plan = clipNarrationFixture().plan
  expect(narrationSlot(plan, 6000)).toEqual({ startMs: 6000, endMs: 8000 })
  // Inside an existing caption: the new one starts after it rather than
  // displacing it, and stops where the next caption begins.
  expect(narrationSlot(plan, 2000)).toEqual({ startMs: 5000, endMs: 7000 })
  const full = applyTimelineEdit(plan, {
    type: 'addNarration',
    id: 'new',
    startMs: 5000,
    endMs: 8000,
  })
  // Every moment up to 12 s now speaks: the caption lands after the last one.
  expect(narrationSlot(full, 6000)).toEqual({ startMs: 12000, endMs: 14000 })
  // A gap too short to read an empty caption in is no room at all.
  const crowded = applyTimelineEdit(full, {
    type: 'addNarration',
    id: 'tail',
    startMs: 12500,
    endMs: plan.durationMs,
  })
  expect(narrationSlot(crowded, 12100)).toBeUndefined()
})

it('adds, retimes and removes a caption on absolute output time', () => {
  const plan = clipNarrationFixture().plan
  const added = applyTimelineEdit(plan, {
    type: 'addNarration',
    id: 'new',
    startMs: 6000,
    endMs: 8000,
  })
  const caption = added.elements!.find((text) => text.instanceId === 'new')!
  expect(caption).toMatchObject({
    narration: true,
    creation: { kind: 'add' },
    cutId: '',
    basis: 'output-start',
    startMs: 6000,
    endMs: 8000,
    text: '',
  })
  const retimed = applyTimelineEdit(added, {
    type: 'text',
    id: 'new',
    patch: { text: '새 자막', startMs: 6500, endMs: 8500 },
  })
  expect(retimed.elements!.find((text) => text.instanceId === 'new')).toMatchObject({
    text: '새 자막',
    startMs: 6500,
    endMs: 8500,
  })
  const removed = applyTimelineEdit(retimed, { type: 'removeText', id: 'new' })
  expect(removed.elements!.some((text) => text.instanceId === 'new')).toBe(false)
  // The plan the edits started from is untouched: undo keeps its own snapshot.
  expect(plan.elements!.some((text) => text.instanceId === 'new')).toBe(false)
})

it('flags a caption past the end or over another one, and never retimes it', () => {
  const plan = clipNarrationFixture().plan
  const beyond = applyTimelineEdit(plan, {
    type: 'text',
    id: 'narration-2',
    patch: { startMs: 19000, endMs: plan.durationMs + 1 },
  })
  expect(nativeTextErrors(beyond).find((e) => e.id === 'narration-2')).toMatchObject({
    interval: true,
  })
  expect(beyond.elements!.find((t) => t.instanceId === 'narration-2')).toMatchObject({
    endMs: plan.durationMs + 1,
  })
  const overlapping = applyTimelineEdit(plan, {
    type: 'text',
    id: 'narration-2',
    patch: { startMs: 4000, endMs: 9000 },
  })
  const errors = nativeTextErrors(overlapping)
  expect(errors.find((e) => e.id === 'narration-2')).toMatchObject({ overlap: true })
  expect(errors.find((e) => e.id === 'narration-1')).toMatchObject({ overlap: true })
  // A plan whose captions merely touch is fine.
  const touching = applyTimelineEdit(plan, {
    type: 'text',
    id: 'narration-2',
    patch: { startMs: 5000, endMs: 9000 },
  })
  expect(nativeTextErrors(touching).every((e) => !e.overlap)).toBe(true)
})

it('reads the narration removal reasons and no longer knows the retired ones', () => {
  for (const code of ['caption_overlap', 'caption_outside_output', 'caption_floor'])
    expect(
      clipNoticeKey({ code, cutId: '', elementId: 'narration-1', action: 'removal' }),
    ).not.toBe('inspection.detailUnknown')
  for (const code of [
    'item_unassigned',
    'item_uncertain',
    'item_binding_conflict',
    'cross_item_identity',
    'context_item_claim',
    'copy_not_generated',
  ])
    expect(clipNoticeKey({ code, cutId: '', elementId: 'x', action: 'removal' })).toBe(
      'inspection.detailUnknown',
    )
})
