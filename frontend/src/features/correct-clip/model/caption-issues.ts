import type { ClipEditableText } from '@/entities/clip-plan'

/** Every problem `nativeTextErrors` can name on one text, in the order the owner fixes them. */
export const CLIP_TEXT_ISSUES = [
  'text',
  'keyword',
  'exposure',
  'interval',
  'regionOverlap',
  'overlap',
  'phrases',
  'size',
  'position',
  'align',
  'accent',
  'identity',
  'stale',
] as const

export type ClipTextIssue = (typeof CLIP_TEXT_ISSUES)[number]

/** The problems one text's validation entry names; none for a valid or unknown text. */
export function textIssues(error?: Partial<Record<ClipTextIssue, boolean>>): ClipTextIssue[] {
  return error ? CLIP_TEXT_ISSUES.filter((key) => error[key]) : []
}

/** A few words of the text, on one line, so an issue says which caption it is (CDS-213). */
export function textExcerpt(text: Pick<ClipEditableText, 'text'>, max = 16) {
  const flat = text.text.replace(/\s+/g, ' ').trim()
  const chars = Array.from(flat)
  return chars.length > max ? `${chars.slice(0, max).join('')}…` : flat
}
