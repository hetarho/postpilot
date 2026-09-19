/** The source binding a composition item carries. It lives with the grammar's own bounds
 *  (`CLIP_COMPOSITION_LIMITS`) rather than with the project, because the plan reads it while
 *  editing cuts and the project reads it as part of its inputs — two nouns, one vocabulary. */
export interface ClipSourceAssociation {
  groupId: string
  itemId: string
  sourceId: string
  fingerprint: string
  startMs: number
  endMs: number
}
