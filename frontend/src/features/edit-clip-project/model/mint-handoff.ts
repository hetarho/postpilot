import { EDITOR_HANDOFF_TTL_MS } from '@/shared/config'
import type { ClipPreparationScreen } from './preparation'

/** What the first save of `/clips/new` hands to the workspace its address change mounts: the
 *  guide screen in use and where the caret was (CLIP-130, CLIP-225). `/clips/new` and
 *  `/clips/$clipId` are different routes, so the mint unmounts the unminted guide; the draft rides
 *  the queue, and the screen and the caret ride this one-shot handoff, as the post's do (POST-11). */
export interface ClipMintHandoff {
  /** The minted id. Only the workspace mounting for this project may claim it. */
  clipId: string
  screen: ClipPreparationScreen
  /** The focused field's element id, with its selection, when the owner was typing. */
  caret?: { field: string; selectionStart: number; selectionEnd: number }
}

let pending: (ClipMintHandoff & { at: number }) | undefined

export function stashClipMintHandoff(handoff: ClipMintHandoff): void {
  pending = { ...handoff, at: Date.now() }
}

/** Reads without consuming, so a component body may call it. The TTL drops a handoff nothing
 *  claimed: a navigation that never happened. */
export function peekClipMintHandoff(clipId: string): ClipMintHandoff | undefined {
  if (!pending || pending.clipId !== clipId) return undefined
  if (Date.now() - pending.at > EDITOR_HANDOFF_TTL_MS) {
    pending = undefined
    return undefined
  }
  return pending
}

export function clearClipMintHandoff(): void {
  pending = undefined
}

/** Where the caret is now, read off the live DOM: a text field inside the given root. */
export function focusedClipField(root: HTMLElement | null): ClipMintHandoff['caret'] | undefined {
  const focused = document.activeElement
  if (
    !root ||
    !(focused instanceof HTMLInputElement || focused instanceof HTMLTextAreaElement) ||
    !focused.id ||
    !root.contains(focused)
  )
    return undefined
  try {
    return {
      field: focused.id,
      selectionStart: focused.selectionStart ?? 0,
      selectionEnd: focused.selectionEnd ?? 0,
    }
  } catch {
    // A number field has no selection to read.
    return { field: focused.id, selectionStart: 0, selectionEnd: 0 }
  }
}

/** Puts the caret back where the handoff says it was, once, and drops the handoff. */
export function restoreClipCaret(clipId: string): void {
  const handoff = peekClipMintHandoff(clipId)
  if (!handoff) return
  clearClipMintHandoff()
  const caret = handoff.caret
  if (!caret) return
  const element = document.getElementById(caret.field)
  if (!(element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement)) return
  element.focus()
  try {
    element.setSelectionRange(caret.selectionStart, caret.selectionEnd)
  } catch {
    /* A number field takes the focus without a selection. */
  }
}
