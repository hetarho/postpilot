import { expect, it, vi } from 'vitest'
import { clipEditingFixture } from '@/test/clip-editing'
import { requiredSourcesForStep, soundRetryAction, unsavedCorrection } from './rules'

it('asks for every source in ① and only the plan’s own sources in ②', () => {
  const plan = clipEditingFixture()
  const draft = { ...plan.plan, cuts: plan.plan.cuts.slice(0, 1) }
  // ① runs the AI over the whole batch, so nothing may be released yet.
  expect(requiredSourcesForStep('generate', draft, plan)).toBeUndefined()
  expect(requiredSourcesForStep('finish', draft, plan)).toBeUndefined()
  // ② draws frames only for the cuts it holds: the second source is released.
  expect(requiredSourcesForStep('refine', draft, plan)?.map((s) => s.id)).toEqual(['a'])
  expect(requiredSourcesForStep('refine', plan.plan, plan)?.map((s) => s.id)).toEqual(['a', 'b'])
  // No saved plan means no cuts to be required for.
  expect(requiredSourcesForStep('refine', draft, undefined)).toBeUndefined()
})

it('re-applies a sound setting the server moved past, and re-sends anything else', () => {
  const correction = { reapply: vi.fn(), save: vi.fn(async () => undefined) }
  soundRetryAction({ ...correction, failure: { reason: 'CLIP_PLAN_CONFLICT' } })()
  expect(correction.reapply).toHaveBeenCalledTimes(1)
  expect(correction.save).not.toHaveBeenCalled()
  soundRetryAction({ ...correction, failure: { reason: 'NETWORK_UNAVAILABLE' } })()
  expect(correction.save).toHaveBeenCalledTimes(1)
  soundRetryAction(correction)()
  expect(correction.save).toHaveBeenCalledTimes(2)
  expect(correction.reapply).toHaveBeenCalledTimes(1)
})

it('holds nothing back once the project is finalized', () => {
  expect(unsavedCorrection(true, undefined)).toBe(true)
  expect(unsavedCorrection(true, { at: '2026-09-20T00:00:00Z' })).toBe(false)
  expect(unsavedCorrection(false, undefined)).toBe(false)
})
