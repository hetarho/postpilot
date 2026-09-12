import { useRef, useState } from 'react'
import {
  useClipLifecycleApi,
  useClipProjectMutations,
  type ClipProject,
} from '@/entities/clip-project'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'

export function useFinalizeClip(
  ownerId: string,
  project: ClipProject,
  flush: () => Promise<number>,
) {
  const api = useClipLifecycleApi(ownerId, project.id)
  const { finalize } = useClipProjectMutations(ownerId)
  const locked = useRef(false)
  const [pending, setPending] = useState(false)
  const [uncertain, setUncertain] = useState(false)
  const [failure, setFailure] = useState<AppFailure>()
  async function reconcile() {
    const found = await api.refresh()
    setUncertain(false)
    if (found.finalized) setFailure(undefined)
    return found
  }
  async function confirm() {
    if (locked.current || uncertain || project.finalized) return
    locked.current = true
    setPending(true)
    setFailure(undefined)
    let sent = false
    try {
      const revision = await flush()
      const saved = await api.refresh()
      if (saved.finalized) return
      if (
        !saved.canFinalize ||
        !saved.result?.id ||
        saved.editPlanRevision !== revision ||
        saved.renderedPlanRevision !== revision
      ) {
        setFailure({ reason: 'CLIP_FINALIZATION_CONFLICT', params: {} })
        return
      }
      sent = true
      await finalize.mutateAsync({
        projectId: saved.id,
        expectedRevision: revision,
        expectedResultId: saved.result.id,
      })
    } catch (error) {
      setFailure(appFailureFromConnect(error))
      if (sent) {
        setUncertain(true)
        try {
          await reconcile()
        } catch {
          /* Keep all mutations locked until an owned read succeeds. */
        }
      }
    } finally {
      locked.current = false
      setPending(false)
    }
  }
  async function checkAgain() {
    if (locked.current) return
    locked.current = true
    setPending(true)
    try {
      await reconcile()
    } catch (error) {
      setFailure(appFailureFromConnect(error))
    } finally {
      locked.current = false
      setPending(false)
    }
  }
  return { confirm, checkAgain, pending, uncertain, failure, busy: pending || uncertain }
}
