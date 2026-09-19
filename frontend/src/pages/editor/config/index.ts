/** How long the editor's cross-route handoff may sit unread
 *  (`pages/editor/model/editor-handoff.ts`). It is meant to be picked up a tick later by
 *  the editor the mint navigation mounts; anything older belongs to a navigation that
 *  never happened, and applying it would put stale text back into a post that has since
 *  moved on. */
export const EDITOR_HANDOFF_TTL_MS = 5_000
