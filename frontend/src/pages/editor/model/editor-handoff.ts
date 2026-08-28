// Where the caret was when the first save moved the URL.
//
// `/posts/new` and `/posts/$slug` are different routes, so minting a slug unmounts the
// new-draft editor and mounts the saved-post one — one second into typing. The text
// survives that on its own (features/save-draft keeps it queued per post), but the caret
// does not: without this the user's next keystroke would go nowhere.
import { EDITOR_HANDOFF_TTL_MS } from '@/shared/config'

export interface CaretHandoff {
  /** The minted slug. Only the editor mounting for this post may claim the handoff. */
  slug: string
  field: 'title' | 'memo'
  selectionStart: number
  selectionEnd: number
}

let pending: (CaretHandoff & { at: number }) | undefined

export function stashCaret(handoff: CaretHandoff): void {
  pending = { ...handoff, at: Date.now() }
}

/** Reads without consuming, so it is safe to call from a component body.
 *
 *  The TTL covers a handoff nothing ever claimed — a navigation that did not happen. Left
 *  unbounded, opening that post days later would yank the caret for no visible reason. */
export function peekCaret(slug: string): CaretHandoff | undefined {
  if (!pending || pending.slug !== slug) return undefined
  if (Date.now() - pending.at > EDITOR_HANDOFF_TTL_MS) {
    pending = undefined
    return undefined
  }
  return pending
}

export function clearCaret(): void {
  pending = undefined
}
