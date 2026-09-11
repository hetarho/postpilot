import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { SAVE_STATUS_SETTLED_MS } from '@/shared/config'
import {
  resolveSaveStatus,
  SAVE_STATUS_LABEL_KEYS,
  type SaveState,
  type SaveStatusState,
} from '@/shared/lib'

export type { SaveStatusState }

/** The PRESENTATION of `SaveState`, and only that.
 *
 *  The queue resolves to `saved` for the whole life of a queue that has ever saved
 *  (`draft-queue.ts`'s `stateOf`), which is correct as a fact and wrong as a message: 저장됨 never
 *  came down, so the one status line could never get round to the post's own status. The settle
 *  lives HERE rather than in the queue on template — the queue's state machine is what autosave
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

  const resolved = resolveSaveStatus(state, settled)
  const key = SAVE_STATUS_LABEL_KEYS[resolved]
  return { state: resolved, label: key ? t(key) : '' }
}
