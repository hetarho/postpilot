/** How long the editor's cross-route caret handoff may sit unread
 *  (`model/caret-handoff.ts`). It is meant to be picked up a tick later by the editor the mint
 *  navigation mounts; anything older belongs to a navigation that never happened, and applying
 *  it would put the caret somewhere the reader has since left. */
export const EDITOR_HANDOFF_TTL_MS = 5_000
