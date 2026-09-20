import { MEMORY_TAGS_MAX, MEMORY_TEXT_MAX_CHARS } from '../config'

/** The closed five (MEM-5). There are no user-defined kinds and no sub-kinds: the kind decides
 *  whether a fact is a candidate for every post or only on tag overlap, and anything finer is a
 *  tag. The order is the one the controls list them in. */
export const MEMORY_KINDS = ['preference', 'persona', 'place', 'person', 'history'] as const

export type MemoryKind = (typeof MEMORY_KINDS)[number]

/** One atomic fact about the author's world, authored by the user — never inferred, scored or
 *  ranked by a model (MEM-23). */
export interface Memory {
  id: string
  text: string
  kind: MemoryKind
  tags: string[]
  /** Every post this fact was approved from, oldest first. Empty for one written by hand. */
  sourcePostSlugs: string[]
  createdAt: string
  updatedAt: string
  /** When this text was last approved again — the retrieval tie-break, and why it is separate
   *  from `updatedAt`, which belongs to an authored edit. */
  lastSeenAt: string
}

export const MEMORY_LIMITS = { text: MEMORY_TEXT_MAX_CHARS, tags: MEMORY_TAGS_MAX } as const

/** Counted the way the backend counts: in Unicode scalar values, so a Hangul syllable is one
 *  character on both sides. `String.length` would count a surrogate pair as two. */
export function memoryChars(value: string): number {
  return [...value.trim()].length
}

export function remainingMemoryChars(value: string): number {
  return MEMORY_LIMITS.text - memoryChars(value)
}

/** The comma-separated tag field as the wire wants it: trimmed, blanks dropped, duplicates
 *  collapsed — the same collapse the server applies, so the count a user sees is the one that
 *  will be checked. */
export function parseMemoryTags(value: string): string[] {
  const seen = new Set<string>()
  for (const raw of value.split(',')) {
    const tag = raw.trim()
    if (tag !== '') seen.add(tag)
  }
  return [...seen]
}

export function formatMemoryTags(tags: readonly string[]): string {
  return tags.join(', ')
}

/** The client-side half of the field rules. It exists to stop an obviously bad save before the
 *  round trip, never to decide one: a value this accepts may still be refused — by the account
 *  cap, or by a text another memory already holds. */
export function canSaveMemory(text: string, tags: readonly string[]): boolean {
  const chars = memoryChars(text)
  return chars > 0 && chars <= MEMORY_LIMITS.text && tags.length <= MEMORY_LIMITS.tags
}
