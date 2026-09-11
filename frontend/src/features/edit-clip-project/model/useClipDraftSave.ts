import { useCallback, useEffect, useState, useSyncExternalStore } from 'react'
import { useTranslation } from 'react-i18next'
import { SAVE_STATUS_SETTLED_MS } from '@/shared/config'
import { resolveSaveStatus, SAVE_STATUS_LABEL_KEYS, type SaveState } from '@/shared/lib'
import { clipDraftState, flushClipDraft, subscribeClipDraft } from './clip-draft-queue'

/** What the clip workspace's ONE status line needs to know about the settings' autosave, and the
 *  flush the credit approval owes itself before it prices a run (CLIP-38, CLIP-39).
 *
 *  Mounted by the PAGE, not by the settings form: ① is a step panel, so the form is gone while
 *  the owner is on ② or ③ and the line still has to report a save that is in flight or failing.
 *
 *  The settle timer is here rather than in the queue for the same reason the post's is in its own
 *  presentation hook: the queue's state machine is what save correctness is tested against, and
 *  how long 저장했어요 stays on screen is not part of it. */
export function useClipDraftSave(projectId: string): {
  state: SaveState
  failing: boolean
  label: string
  flush: () => Promise<void>
} {
  const { t } = useTranslation('common')
  const subscribe = useCallback(
    (listener: () => void) => subscribeClipDraft(projectId, listener),
    [projectId],
  )
  const state = useSyncExternalStore(
    subscribe,
    () => clipDraftState(projectId),
    () => 'idle' as SaveState,
  )

  const [settled, setSettled] = useState(false)
  // Derived state adjusted DURING render rather than from an effect: an effect would paint one
  // frame of a settled 저장했어요 again before it corrected itself.
  const [reported, setReported] = useState(state)
  if (reported !== state) {
    setReported(state)
    if (settled) setSettled(false)
  }
  useEffect(() => {
    if (state !== 'saved') return
    const timer = setTimeout(() => setSettled(true), SAVE_STATUS_SETTLED_MS)
    return () => clearTimeout(timer)
  }, [state])

  const resolved = resolveSaveStatus(state, settled)
  const key = SAVE_STATUS_LABEL_KEYS[resolved]
  return {
    state,
    failing: resolved === 'error',
    label: key ? t(key) : '',
    // Keyed by the id it was called with: the workspace is mounted per project, so this identity
    // only changes when the whole page does.
    flush: useCallback(() => flushClipDraft(projectId), [projectId]),
  }
}
