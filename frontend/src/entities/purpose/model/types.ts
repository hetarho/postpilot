import i18next from 'i18next'
import {
  PURPOSE_DESCRIPTION_MAX_CHARS,
  PURPOSE_INSTRUCTIONS_MAX_CHARS,
  PURPOSE_NAME_MAX_CHARS,
} from '@/shared/config'

/** A reusable 용도 brief (spec/policy/purposes.md): what a kind of post is for and how that
 *  kind must be written. Authored text only — nothing here is learned or inferred, and the
 *  voice profile is untouched by it. */
export interface Purpose {
  id: string
  name: string
  /** May be empty. Shown as help text under the selector and injected as the 이 글의 용도 line. */
  description: string
  instructions: string
  /** Posts currently assigned to it. A projection: the delete confirmation names it. */
  postCount: number
  createdAt: string
  updatedAt: string
}

/** The purpose a post is assigned to, as a post screen needs it. An empty id is 없음 — the
 *  default, and a real answer rather than a missing value. */
export interface PurposeRef {
  id: string
  name: string
}

/** Mirrored from the backend so the fields can count down before the round trip; the server
 *  stays authoritative and its message is what a refusal shows. */
export const PURPOSE_LIMITS = {
  name: PURPOSE_NAME_MAX_CHARS,
  description: PURPOSE_DESCRIPTION_MAX_CHARS,
  instructions: PURPOSE_INSTRUCTIONS_MAX_CHARS,
} as const

/** How the empty purpose is written wherever it can be chosen or shown. */
export function noPurposeLabel(): string {
  return i18next.t('noPurpose', { ns: 'purposes' })
}

/** The `<option>` value standing for 없음. Empty, because that is exactly what the wire
 *  carries to clear an assignment (a present empty `purpose_id`). */
export const NO_PURPOSE_VALUE = ''

export function emptyPurposeRef(): PurposeRef {
  return { id: '', name: '' }
}

/** Counted the way the backend counts: in Unicode scalar values, so a Hangul syllable is one
 *  character on both sides. `String.length` would count a surrogate pair as two. */
export function purposeChars(value: string): number {
  return [...value.trim()].length
}

export function remainingChars(value: string, max: number): number {
  return max - purposeChars(value)
}

/** The client-side half of the field rules. It exists to stop an obviously bad save before
 *  the round trip, never to decide one: a value this accepts may still be refused. */
export function canSavePurpose(fields: {
  name: string
  description: string
  instructions: string
}): boolean {
  return (
    purposeChars(fields.name) > 0 &&
    purposeChars(fields.name) <= PURPOSE_LIMITS.name &&
    purposeChars(fields.description) <= PURPOSE_LIMITS.description &&
    purposeChars(fields.instructions) > 0 &&
    purposeChars(fields.instructions) <= PURPOSE_LIMITS.instructions
  )
}

/** What the delete confirmation says. The count comes from the server, and the delete
 *  detaches rather than cascading: no post and no content is removed. */
export function detachWarning(postCount: number): string {
  return postCount > 0
    ? i18next.t('detachWarning.used', { ns: 'purposes', count: postCount })
    : i18next.t('detachWarning.unused', { ns: 'purposes' })
}
