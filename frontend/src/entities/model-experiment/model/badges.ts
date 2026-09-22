import type { StageName } from '@/entities/model-catalog/@x/model-experiment'

/** The fixed badge catalog a verdict may attach to each candidate. It is code-owned and its
 *  ids are stable, because a leaderboard tallies them across releases; the copy for each one
 *  lives in this slice's locale fragment. */
export type VerdictBadgeName =
  | 'fast'
  | 'natural'
  | 'on_brief'
  | 'structured'
  | 'accurate'
  | 'in_voice'
  | 'concise'
  | 'slow'
  | 'ai_like'
  | 'off_brief'
  | 'verbose'
  | 'inaccurate'
  | 'off_voice'
  | 'repetitive'
  | 'broken_format'
  | 'other'

/** The two groups the sheet presents, in the order it presents them. `other` belongs to
 *  neither and is rendered below both, because it carries a note rather than a judgement. */
export const POSITIVE_BADGES = [
  'fast',
  'natural',
  'on_brief',
  'structured',
  'accurate',
  'in_voice',
  'concise',
] as const satisfies readonly VerdictBadgeName[]

export const NEGATIVE_BADGES = [
  'slow',
  'ai_like',
  'off_brief',
  'verbose',
  'inaccurate',
  'off_voice',
  'repetitive',
  'broken_format',
] as const satisfies readonly VerdictBadgeName[]

/** The free note beside `other`, bounded the same way the server bounds it — in characters,
 *  so Korean prose is not cut to a third of what the field promises. */
export const BADGE_NOTE_MAX_LENGTH = 200

/** Whether a stage offers this badge. Only the voice pair is conditional: an observe
 *  comparison produces no prose, so neither judgement about voice can be made of it. */
export function badgeAppliesTo(badge: VerdictBadgeName, stage: StageName): boolean {
  if (badge === 'in_voice' || badge === 'off_voice') return stage === 'write' || stage === 'analyze'
  return true
}

/** Whether a badge reads as praise. Used for the tone a chip and a revealed badge carry. */
export function isPositiveBadge(badge: VerdictBadgeName): boolean {
  return (POSITIVE_BADGES as readonly VerdictBadgeName[]).includes(badge)
}

/** What one candidate was given at the verdict. Zero badges is as valid as ten. */
export interface CandidateBadges {
  candidateId: string
  badges: VerdictBadgeName[]
  otherNote: string
}
