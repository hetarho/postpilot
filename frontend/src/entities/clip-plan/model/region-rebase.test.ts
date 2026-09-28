import { expect, it } from 'vitest'
import { clipTimelineFixture } from '@/test/clip-editing'
import type { ClipEditPlan, ClipEditableText } from './edit-plan'
import { clipRegionRows, rebaseClipRegions } from './region-rebase'
import { clipTimelineReducer, createClipTimeline } from './timeline'

const intro = (...rows: string[]): ClipEditableText => ({
  instanceId: 'project-intro',
  elementId: 'project-intro',
  cutId: '',
  kind: 'fixed',
  role: 'hook',
  text: '',
  rows: rows.map((text) => ({ role: 'headline', text })),
  style: 'auto',
  position: 'auto',
  align: 'center',
  basis: 'output-start',
  startMs: 0,
  endMs: 2500,
  pace: '',
  accent: '',
  keyword: '',
  resolvedStartMs: 0,
  resolvedEndMs: 2500,
  groupId: '',
  itemId: '',
})
const withIntro = (plan: ClipEditPlan, element?: ClipEditableText): ClipEditPlan => ({
  ...plan,
  elements: [
    ...(element ? [element] : []),
    ...(plan.elements ?? []).filter((t) => t.role !== 'hook'),
  ],
})
const recaption = (plan: ClipEditPlan, text: string): ClipEditPlan => ({
  ...plan,
  elements: plan.elements?.map((t) => (t.instanceId === 'caption-a' ? { ...t, text } : t)),
})

const base = withIntro(clipTimelineFixture().plan, intro('성수 로컬', ''))

// CLIP-188: a slot save projects into the plan under a correction the owner is still typing.
// The draft takes the new region words and keeps its own caption edit.
it('carries a correction onto a plan whose regions alone moved', () => {
  const local = recaption(base, '고친 자막')
  const server = withIntro(base, intro('성수 로컬', '저녁 영업'))
  const merged = rebaseClipRegions(local, base, server)
  expect(merged && clipRegionRows(merged, 'intro')).toEqual(['성수 로컬', '저녁 영업'])
  expect(merged?.elements?.find((t) => t.instanceId === 'caption-a')?.text).toBe('고친 자막')
})

it('takes a region the server turned on or off', () => {
  const local = recaption(base, '고친 자막')
  expect(clipRegionRows(rebaseClipRegions(local, base, withIntro(base))!, 'intro')).toBeUndefined()
  const off = withIntro(base)
  const on = rebaseClipRegions(recaption(off, '고친 자막'), off, base)
  expect(on && clipRegionRows(on, 'intro')).toEqual(['성수 로컬', ''])
})

// A row the owner typed and a row the slot save wrote join; the same row written two ways is a
// real conflict, left for the owner (CLIP-39).
it('joins rows both sides changed, and refuses one both changed differently', () => {
  const typed = withIntro(base, intro('내가 쓴 첫 줄', ''))
  const saved = withIntro(base, intro('성수 로컬', '저녁 영업'))
  expect(clipRegionRows(rebaseClipRegions(typed, base, saved)!, 'intro')).toEqual([
    '내가 쓴 첫 줄',
    '저녁 영업',
  ])
  const clash = withIntro(base, intro('다른 첫 줄', ''))
  expect(rebaseClipRegions(typed, base, clash)).toBeUndefined()
})

// Anything else the server moved — another tab's correction, a revision job — is a conflict,
// and so is a revision that moved nothing it could take.
it('refuses a new revision that is not a region projection', () => {
  const local = recaption(base, '고친 자막')
  expect(rebaseClipRegions(local, base, recaption(base, '다른 탭의 자막'))).toBeUndefined()
  expect(rebaseClipRegions(local, base, base)).toBeUndefined()
})

// The history follows the slots as well, so an undo never brings old region words back.
it('moves the history onto the server regions when adopting with them', () => {
  let state = createClipTimeline(base)
  state = clipTimelineReducer(state, {
    type: 'edit',
    edit: { type: 'text', id: 'caption-a', patch: { text: '고친 자막' } },
    at: 0,
  })
  const server = withIntro(base, intro('성수 로컬', '저녁 영업'))
  const merged = rebaseClipRegions(state.plan, base, server)!
  state = clipTimelineReducer(state, { type: 'adopt', plan: merged, regionsFrom: server })
  state = clipTimelineReducer(state, { type: 'undo' })
  expect(state.plan.elements?.find((t) => t.instanceId === 'caption-a')?.text).toBe('caption a')
  expect(clipRegionRows(state.plan, 'intro')).toEqual(['성수 로컬', '저녁 영업'])
})
