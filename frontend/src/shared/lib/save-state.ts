/** What a debounced save queue is doing. Shared because two features now autosave — the post
 *  draft and the clip project's settings — and the one status line each of them reports through
 *  has to speak the same five words (CLIP-38, POST-45). */
export type SaveState = 'idle' | 'dirty' | 'saving' | 'saved' | 'error'

/** What a status line has to SAY about that, once `saved` has been allowed to settle. `quiet` is
 *  "nothing", and it is a state no queue itself has. */
export type SaveStatusState = 'quiet' | 'dirty' | 'saving' | 'saved' | 'error'

/** A queue resolves to `saved` for the whole life of a queue that has ever saved, which is correct
 *  as a fact and wrong as a message: 저장됨 would never come down and the one status line could
 *  never get round to the object's own state. The settle is presentation, so it lives here rather
 *  than in either queue's state machine — that machine is what save correctness is tested against,
 *  and "how long a word stays on screen" is not part of it. */
export function resolveSaveStatus(state: SaveState, settled: boolean): SaveStatusState {
  return state === 'idle' || (state === 'saved' && settled) ? 'quiet' : state
}

/** The `common` namespace key each resolved state reads, `null` for silence. Keys rather than
 *  copy, so this file stays react- and i18next-free (ARCH-18), and `as const` so the caller's
 *  `t()` still sees the literal keys its own typing demands. */
export const SAVE_STATUS_LABEL_KEYS = {
  quiet: null,
  dirty: 'state.savePending',
  saving: 'action.saving',
  saved: 'state.saved',
  error: 'state.saveRetrying',
} as const satisfies Record<SaveStatusState, string | null>
