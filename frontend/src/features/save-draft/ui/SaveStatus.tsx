import { Notice } from '@/shared/ui'
import type { SaveState } from '../model/draft-queue'

/** Nothing at all while the editor is untouched: a "저장됨" on a post nobody has edited
 *  yet would claim a save that never happened. */
const LABELS: Record<SaveState, string> = {
  idle: '',
  dirty: '저장 대기 중',
  saving: '저장 중…',
  saved: '저장됨',
  error: '저장하지 못했어요 · 다시 시도 중',
}

export function SaveStatus({ state }: { state: SaveState }) {
  return (
    // The live region is this wrapper and only its CONTENTS change. A region that is inserted
    // already holding its message is not reliably announced, so swapping the element itself for
    // the failure state would silence exactly the state that matters most.
    //
    // Polite, not assertive: this changes about once a second while someone types, and a
    // screen reader interrupting every keystroke would make the editor unusable.
    <div role="status" aria-live="polite" data-state={state}>
      {state === 'error' ? (
        // A failed autosave is the only thing this screen has to say instead of a save button
        // (PRD F-2), so it takes the §2.6 notice contract rather than 12px of bare red text.
        <Notice tone="danger">{LABELS.error}</Notice>
      ) : (
        <p
          className={
            state === 'saved' ? 'text-notice-success-fg text-xs' : 'text-content-tertiary text-xs'
          }
        >
          {LABELS[state]}
        </p>
      )}
    </div>
  )
}
