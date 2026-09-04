import i18next from 'i18next'
import {
  TEMPLATE_DESCRIPTION_MAX_CHARS,
  TEMPLATE_BODY_MAX_CHARS,
  TEMPLATE_NAME_MAX_CHARS,
} from '@/shared/config'

/** A reusable 템플릿 brief (spec/policy/templates.md): what a kind of post is for and how that
 *  kind must be written. Authored text only — nothing here is learned or inferred, and the
 *  voice profile is untouched by it. */
export interface Template {
  id: string
  name: string
  /** May be empty. Shown as help text under the selector and injected as the 이 글의 템플릿 line. */
  description: string
  body: string
  /** Posts currently assigned to it. A projection: the delete confirmation names it. */
  postCount: number
  createdAt: string
  updatedAt: string
}

/** The template a post is assigned to, as a post screen needs it. An empty id is 없음 — the
 *  default, and a real answer rather than a missing value. */
export interface TemplateRef {
  id: string
  name: string
}

/** Mirrored from the backend so the fields can count down before the round trip; the server
 *  stays authoritative and its message is what a refusal shows. */
export const TEMPLATE_LIMITS = {
  name: TEMPLATE_NAME_MAX_CHARS,
  description: TEMPLATE_DESCRIPTION_MAX_CHARS,
  body: TEMPLATE_BODY_MAX_CHARS,
} as const

/** How the empty template is written wherever it can be chosen or shown. */
export function noTemplateLabel(): string {
  return i18next.t('noTemplate', { ns: 'templates' })
}

/** The `<option>` value standing for 없음. Empty, because that is exactly what the wire
 *  carries to clear an assignment (a present empty `template_id`). */
export const NO_TEMPLATE_VALUE = ''

export function emptyTemplateRef(): TemplateRef {
  return { id: '', name: '' }
}

/** Counted the way the backend counts: in Unicode scalar values, so a Hangul syllable is one
 *  character on both sides. `String.length` would count a surrogate pair as two. */
export function templateChars(value: string): number {
  return [...value.trim()].length
}

export function remainingChars(value: string, max: number): number {
  return max - templateChars(value)
}

/** The client-side half of the field rules. It exists to stop an obviously bad save before
 *  the round trip, never to decide one: a value this accepts may still be refused. */
export function canSaveTemplate(fields: {
  name: string
  description: string
  body: string
}): boolean {
  return (
    templateChars(fields.name) > 0 &&
    templateChars(fields.name) <= TEMPLATE_LIMITS.name &&
    templateChars(fields.description) <= TEMPLATE_LIMITS.description &&
    templateChars(fields.body) > 0 &&
    templateChars(fields.body) <= TEMPLATE_LIMITS.body
  )
}

/** What the delete confirmation says. The count comes from the server, and the delete
 *  detaches rather than cascading: no post and no content is removed. */
export function detachWarning(postCount: number): string {
  return postCount > 0
    ? i18next.t('detachWarning.used', { ns: 'templates', count: postCount })
    : i18next.t('detachWarning.unused', { ns: 'templates' })
}
