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
    <p
      // Polite, not assertive: this changes about once a second while someone types, and a
      // screen reader interrupting every keystroke would make the editor unusable.
      role="status"
      aria-live="polite"
      data-state={state}
      className={
        state === 'error'
          ? 'text-notice-danger-fg text-xs'
          : state === 'saved'
            ? 'text-notice-success-fg text-xs'
            : 'text-content-tertiary text-xs'
      }
    >
      {LABELS[state]}
    </p>
  )
}
