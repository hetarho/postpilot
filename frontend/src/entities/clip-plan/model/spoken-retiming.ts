import {
  copyClipPlan,
  clipPlanDuration,
  cutRate,
  timelineCuts,
  transformedDurationMs,
  type ClipEditPlan,
  type ClipEditingState,
} from './edit-plan'
import { clipDraftKey, textInterval, validateTimelinePlan, type ClipCutEvidence } from './timeline'
import { speechDurationMs, spokenState } from './spoken'
export type SpokenRetiming =
  | { available: false; reason: 'speech' | 'duration' | 'footage' | 'captions' }
  | {
      available: true
      plan: ClipEditPlan
      expectedKey: string
      changedCuts: string[]
      changedCaptions: string[]
    }
/** A conservative proposal: existing source rates and caption output intervals stay fixed.
 * Extending observed unused footage is explicit; unavailable capacity never loops a source. */
export function proposeSpokenRetiming(
  plan: ClipEditPlan,
  state: ClipEditingState,
  evidence: ClipCutEvidence | undefined,
  targetMs = 60000,
  styles: readonly string[] = [],
): SpokenRetiming {
  const n = plan.narration
  if (
    !n?.segments.length ||
    n.segments.some((s) => ['missing', 'stale'].includes(spokenState(n, s)))
  )
    return { available: false, reason: 'speech' }
  const next = copyClipPlan(plan),
    narration = next.narration!
  let intro = 0,
    outro = 0
  for (const text of plan.elements ?? []) {
    const interval = textInterval(plan, text)
    if (text.role === 'hook' && interval.valid) intro = Math.max(intro, interval.endMs)
    if (text.role === 'ending' && interval.valid)
      outro = Math.max(outro, plan.durationMs - interval.startMs)
  }
  let cursor = intro
  for (const s of narration.segments) {
    s.startMs = Math.max(cursor, s.startMs)
    s.endMs = s.startMs + speechDurationMs(s.speech!)
    cursor = s.endMs
  }
  const desired = Math.max(15000, plan.durationMs, cursor + outro)
  if (desired > Math.min(60000, targetMs)) return { available: false, reason: 'duration' }
  const changedCuts: string[] = [],
    changedCaptions: string[] = []
  if (desired > plan.durationMs) {
    if (evidence?.status !== 'available') return { available: false, reason: 'footage' }
    for (let i = next.cuts.length - 1; i >= 0 && clipPlanDuration(next.cuts) < desired; i--) {
      const cut = next.cuts[i],
        original = plan.cuts[i]
      const observed = evidence.sources
        .find((s) => s.source.id === cut.sourceId && s.source.fingerprint === cut.fingerprint)
        ?.segments.find(
          (s) => s.usability !== 'unusable' && s.startMs <= cut.startMs && s.endMs >= cut.endMs,
        )
      if (!observed) continue
      const occupied = next.cuts
        .filter(
          (c) =>
            c.id !== cut.id &&
            c.sourceId === cut.sourceId &&
            c.fingerprint === cut.fingerprint &&
            c.startMs >= cut.endMs,
        )
        .map((c) => c.startMs)
      const maximum = Math.min(observed.endMs, ...occupied)
      const missing = desired - clipPlanDuration(next.cuts)
      cut.endMs = Math.min(maximum, cut.endMs + Math.ceil((missing * cutRate(cut)) / 1000))
      if (
        transformedDurationMs(cut.endMs - cut.startMs, cutRate(cut)) >
        transformedDurationMs(original.endMs - original.startMs, cutRate(cut))
      )
        changedCuts.push(cut.id)
    }
    next.durationMs = clipPlanDuration(next.cuts)
    if (next.durationMs < desired || next.durationMs > Math.min(60000, targetMs))
      return { available: false, reason: 'footage' }
    for (const text of next.elements ?? []) {
      if (text.role !== 'caption') continue
      const prior = plan.elements!.find((t) => t.instanceId === text.instanceId)!,
        interval = textInterval(plan, prior)
      if (!interval.valid) return { available: false, reason: 'captions' }
      if (text.basis === 'cut' || text.basis === 'output-end' || text.basis === 'whole') {
        text.basis = 'output-start'
        text.startMs = interval.startMs
        text.endMs = interval.endMs
        text.ownerEdited = true
        if (text.derivedCaption) text.derivedCaption.timingEdited = true
        changedCaptions.push(text.instanceId)
      }
    }
  }
  if (!validateTimelinePlan(next, state, evidence, styles).saveable)
    return { available: false, reason: 'captions' }
  // Region exclusions and transitions must leave every measured sentence intact.
  const ending = Math.min(
    ...(next.elements ?? [])
      .filter((t) => t.role === 'ending')
      .map((t) => textInterval(next, t).startMs),
    next.durationMs,
  )
  if (cursor > ending || !timelineCuts(next).length) return { available: false, reason: 'captions' }
  return {
    available: true,
    plan: next,
    expectedKey: clipDraftKey(plan),
    changedCuts,
    changedCaptions,
  }
}
