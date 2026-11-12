import { useRef, useState } from 'react'
import { type ClipProject, useClipLifecycleApi } from '@/entities/clip-project'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'

/** 확정하기 (CLIP-75): flushes every queue the owner types into, confirms the plan revision that
 *  flush saved and hands the confirmed project on. There is no dialog: confirmation renders
 *  nothing, deletes nothing and ends no editing, and the next changed plan save reverses it
 *  (CLIP-229). */
export function useConfirmClip({
  ownerId,
  project,
  flush,
  onConfirmed,
}: {
  ownerId: string
  project: ClipProject
  /** Writes the settings, regions, storyline and correction queues and answers with the plan
   *  revision they landed on (CLIP-39). */
  flush: () => Promise<number>
  onConfirmed: (project: ClipProject) => void
}) {
  const api = useClipLifecycleApi(ownerId, project.id)
  const locked = useRef(false)
  const [pending, setPending] = useState(false)
  const [failure, setFailure] = useState<AppFailure>()
  async function confirm() {
    if (locked.current || project.finalized) return
    locked.current = true
    setPending(true)
    setFailure(undefined)
    try {
      const revision = await flush()
      onConfirmed(await api.confirm(revision))
    } catch (error) {
      // A failed save states itself on the status line too; the plan stays unconfirmed.
      setFailure(appFailureFromConnect(error))
    } finally {
      locked.current = false
      setPending(false)
    }
  }
  return { confirm, pending, failure }
}
