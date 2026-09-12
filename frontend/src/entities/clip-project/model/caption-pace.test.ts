import { expect, it } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { ClipEditingStateSchema } from '@/shared/api'
import { clipEditingFixture } from '@/test/clip-editing'
import { clipPlanToProto, toClipEditingState } from '../api/edit-plan'
import { editClipPlan, validateClipPlan } from './edit-plan'
import { splitRapid, rapidPhrases } from './caption-pace'

it('splits the requested Korean example into exactly 300/500/500 ms and preserves text', () => {
  const seed = {
    ...clipEditingFixture().plan.cuts[0]!.copies[0]!,
    text: '오늘은 구로디지털단지에 와보았는데요',
  }
  const cues = splitRapid(seed, 120, 1420)!
  expect(cues.map((c) => [c.text, c.startMs, c.endMs, c.pace])).toEqual([
    ['오늘은', 120, 420, 'rapid'],
    ['구로디지털단지에', 420, 920, 'rapid'],
    ['와보았는데요', 920, 1420, 'rapid'],
  ])
  expect(rapidPhrases('오늘은 철판 요리를 먹어봤어요')).toEqual([
    '오늘은',
    '철판 요리를',
    '먹어봤어요',
  ])
  expect(splitRapid(seed, 0, 899)).toBeNull()
  const compressed = splitRapid(seed, 0, 1000)!
  expect(compressed.at(-1)!.endMs).toBeLessThanOrEqual(1000)
  expect(compressed.every((c) => c.endMs - c.startMs >= 300)).toBe(true)
  expect(rapidPhrases('가'.repeat(30))).toEqual(['가'.repeat(14), '가'.repeat(14), '가가'])
})

it('splits, edits, adds/removes, persists and merges without changing the original plan', () => {
  const state = clipEditingFixture()
  state.plan.cuts[0]!.copies[0]!.text = '오늘은 구로디지털단지에 와보았는데요'
  const original = structuredClone(state.plan),
    id = state.plan.cuts[0]!.id
  let plan = editClipPlan(state.plan, { type: 'pace', id, pace: 'rapid' })
  expect(validateClipPlan(plan, state).valid).toBe(true)
  const proto = create(ClipEditingStateSchema, { ...state, plan: clipPlanToProto(plan) })
  expect(toClipEditingState(proto).plan).toEqual(plan)
  plan = editClipPlan(plan, { type: 'addCopy', id })
  expect(plan.cuts[0]!.copies).toHaveLength(4)
  expect(validateClipPlan(plan, state).valid).toBe(false)
  plan = editClipPlan(plan, { type: 'copy', id, index: 3, patch: { text: '다음에는' } })
  expect(validateClipPlan(plan, state).valid).toBe(true)
  plan = editClipPlan(plan, { type: 'removeCopy', id, index: 1 })
  expect(plan.cuts[0]!.copies.map((c) => c.text)).toEqual(['오늘은', '와보았는데요', '다음에는'])
  plan = editClipPlan(plan, { type: 'pace', id, pace: 'steady' })
  expect(plan.cuts[0]!.copies).toHaveLength(1)
  expect(plan.cuts[0]!.copies[0]!.text).toBe('오늘은 와보았는데요 다음에는')
  expect(plan.cuts[0]!.copies[0]!.startMs).toBe(0)
  expect(state.plan).toEqual(original)
})

it.each(['overlap', 'short', 'long', 'implicit', 'mixed', 'empty', 'lines'])(
  'refuses invalid rapid cue: %s',
  (invalid) => {
    const state = clipEditingFixture(),
      id = state.plan.cuts[0]!.id
    state.plan.cuts[0]!.copies[0]!.text = '오늘은 구로디지털단지에 와보았는데요'
    const plan = editClipPlan(state.plan, { type: 'pace', id, pace: 'rapid' })
    const cues = plan.cuts[0]!.copies
    if (invalid === 'overlap') cues[1]!.startMs = 419
    if (invalid === 'short') cues[0]!.endMs = 419
    if (invalid === 'long') cues[2]!.endMs = cues[2]!.startMs + 1001
    if (invalid === 'implicit') {
      cues[0]!.startMs = 0
      cues[0]!.endMs = 0
    }
    if (invalid === 'mixed') cues[0]!.pace = 'steady'
    if (invalid === 'empty') cues[0]!.text = ''
    if (invalid === 'lines') cues[0]!.text = '오늘은\n여기로'
    expect(validateClipPlan(plan, state).valid).toBe(false)
  },
)
