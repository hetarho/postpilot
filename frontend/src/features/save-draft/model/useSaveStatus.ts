import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { SAVE_STATUS_SETTLED_MS } from '@/shared/config'
import type { SaveState } from './draft-queue'

/** What the status line has to say about the save, once `saved` has been allowed to settle.
 *  `quiet` is "nothing", and it is a state the queue itself does not have. */
export type SaveStatusState = 'quiet' | 'dirty' | 'saving' | 'saved' | 'error'

/** The PRESENTATION of `SaveState`, and only that.
 *
 *  The queue resolves to `saved` for the whole life of a queue that has ever saved
 *  (`draft-queue.ts`'s `stateOf`), which is correct as a fact and wrong as a message: 저장됨 never
 *  came down, so the one status line could never get round to the post's own status. The settle
 *  lives HERE rather than in the queue on purpose — the queue's state machine is what autosave
 *  correctness is tested against, and "how long a word stays on screen" is not part of it
 *  (tech/draft-autosave.md).
 *
 *  Every state change re-arms the timer, so a save that follows a settled one is announced again. */
export function useSaveStatus(state: SaveState): { state: SaveStatusState; label: string } {
  const { t } = useTranslation('common')
  const [settled, setSettled] = useState(false)
  // Derived state, adjusted DURING render rather than from an effect: an effect would paint one
  // frame of a settled 저장됨 again before it corrected itself.
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

  const resolved: SaveStatusState =
    state === 'idle' || (state === 'saved' && settled) ? 'quiet' : state
  const label = {
    quiet: '',
    dirty: t('state.savePending'),
    saving: t('action.saving'),
    saved: t('state.saved'),
    error: t('state.saveRetrying'),
  }[resolved]
  return { state: resolved, label }
}
