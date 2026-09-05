import { GUIDELINE_TEXT_MAX_CHARS } from '@/shared/config'

/** What a guideline applies to (spec/policy/guidelines.md). `templates` with an empty set is a
 *  real state, not a missing value: every template it named was deleted, so it reaches no post
 *  until it is rescoped. */
export type GuidelineScopeKind = 'global' | 'templates'

/** A template a guideline is scoped to, projected by name. A read projection through the template
 *  directory — the guideline itself stores only ids. */
export interface GuidelineTemplateRef {
  id: string
  name: string
}

/** A saved 작문 지침: one authored rule about what a post must avoid or watch out for.
 *  Authored text only — nothing here is learned, inferred, or written by a model. */
export interface Guideline {
  id: string
  text: string
  scope: GuidelineScopeKind
  /** Empty for `global`, and also empty for an orphaned `templates` scope. */
  templates: GuidelineTemplateRef[]
  createdAt: string
  updatedAt: string
}

/** A recorded revision instruction awaiting review (change 26). It is a receipt for something the
 *  user typed — nothing rewrites, summarizes, generalizes or ranks it, and it reaches no prompt
 *  until it is approved as a guideline.
 *
 *  It carries no scope: scope is a durable decision about every future post, so it is made at
 *  approval time. That absence is what lets recording be automatic. */
export interface GuidelineCandidate {
  id: string
  /** Stored at the REVISION bound (500), which is wider than a guideline's, so a long correction
   *  is kept rather than refused. Approving one past the guideline bound is refused by the
   *  server, which is why a candidate opens for editing with the live count. */
  text: string
  /** Empty when the source post was deleted: the text survives, the link does not. */
  postSlug: string
  /** How many completed revisions carried this exact text — the signal that a one-off correction
   *  has become a standing rule. */
  occurrences: number
  firstSeenAt: string
  lastSeenAt: string
}

/** The whole scope as one value, because a scope is a kind plus a set: an edit either leaves it
 *  alone or replaces both halves together. */
export interface GuidelineScope {
  kind: GuidelineScopeKind
  templateIds: string[]
}

/** Mirrored from the backend so the field can count down before the round trip; the server stays
 *  authoritative and its message is what a refusal shows. The per-account cap is deliberately not
 *  mirrored — the create form relays the server's refusal rather than predicting it. */
export const GUIDELINE_LIMITS = { text: GUIDELINE_TEXT_MAX_CHARS } as const

export function globalScope(): GuidelineScope {
  return { kind: 'global', templateIds: [] }
}

/** Counted the way the backend counts: in Unicode scalar values, so a Hangul syllable is one
 *  character on both sides. `String.length` would count a surrogate pair as two. */
export function guidelineChars(value: string): number {
  return [...value.trim()].length
}

export function remainingGuidelineChars(value: string): number {
  return GUIDELINE_LIMITS.text - guidelineChars(value)
}

/** True when a scoped guideline reaches no post at all — every template it named was deleted. */
export function isOrphanedScope(guideline: Pick<Guideline, 'scope' | 'templates'>): boolean {
  return guideline.scope === 'templates' && guideline.templates.length === 0
}

/** The client-side half of the field rules. It exists to stop an obviously bad save before the
 *  round trip, never to decide one: a value this accepts may still be refused. */
export function canSaveGuideline(text: string, scope: GuidelineScope): boolean {
  const chars = guidelineChars(text)
  if (chars === 0 || chars > GUIDELINE_LIMITS.text) return false
  return scope.kind === 'global' ? scope.templateIds.length === 0 : scope.templateIds.length > 0
}
