import {
  type GenerationOptionsSet,
  POST_TAG_COUNT_MAX,
  POST_TAG_COUNT_MIN,
  POST_TARGET_LENGTH_MAX,
  POST_TARGET_LENGTH_MIN,
} from '@/entities/post'

/** The writing brief's run options as the form holds them while it is open (POST-89). The two
 *  numbers stay strings, as typed, so an empty or half-typed field is representable. */
export interface RunOptionsDraft {
  lengthOn: boolean
  length: string
  tags: string
  useMemory: boolean
  qualityRules: GenerationOptionsSet['qualityRules']
  field: GenerationOptionsSet['field']
}

/** The form an opening of the brief seeds from the post's saved set. */
export function draftFromSet(set: GenerationOptionsSet): RunOptionsDraft {
  return {
    lengthOn: set.targetLength !== undefined,
    length: set.targetLength?.toString() ?? '',
    tags: String(set.tagCount),
    useMemory: set.useMemory,
    qualityRules: set.qualityRules,
    field: set.field,
  }
}

/** Natural length (the box off) is always valid; a ticked length is an integer in range. */
export function lengthValid(draft: RunOptionsDraft): boolean {
  if (!draft.lengthOn) return true
  const parsed = Number(draft.length)
  return (
    Number.isInteger(parsed) && parsed >= POST_TARGET_LENGTH_MIN && parsed <= POST_TARGET_LENGTH_MAX
  )
}

/** The tag count is always a number in range (POST-63). */
export function tagsValid(draft: RunOptionsDraft): boolean {
  const parsed = Number(draft.tags)
  return (
    draft.tags !== '' &&
    Number.isInteger(parsed) &&
    parsed >= POST_TAG_COUNT_MIN &&
    parsed <= POST_TAG_COUNT_MAX
  )
}

/** The set a valid form saves: natural length when the box is off. */
export function setFromDraft(draft: RunOptionsDraft): GenerationOptionsSet {
  return {
    targetLength: draft.lengthOn ? Number(draft.length) : undefined,
    tagCount: Number(draft.tags),
    useMemory: draft.useMemory,
    qualityRules: draft.qualityRules,
    field: draft.field,
  }
}

/** Whether the form holds a valid set that differs from the saved one, the ticks compared as
 *  sets. Compared through the set, not the raw form: a length ticked on and off again, or a
 *  number retyped to what it was, is no change. */
export function changedFrom(draft: RunOptionsDraft, saved: GenerationOptionsSet): boolean {
  if (!lengthValid(draft) || !tagsValid(draft)) return false
  const next = setFromDraft(draft)
  const ticks = new Set(next.qualityRules)
  return (
    next.targetLength !== saved.targetLength ||
    next.tagCount !== saved.tagCount ||
    next.useMemory !== saved.useMemory ||
    next.field !== saved.field ||
    ticks.size !== new Set(saved.qualityRules).size ||
    saved.qualityRules.some((id) => !ticks.has(id))
  )
}
