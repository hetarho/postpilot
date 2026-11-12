import { useRef, useState } from 'react'
import { type ClipProject, useClipLifecycleApi } from '@/entities/clip-project'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'

export function useFinalizeClip(
  ownerId: string,
  project: ClipProject,
  flush: () => Promise<number>,
) {
  const api = useClipLifecycleApi(ownerId, project.id)
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
  /** Flush and read before showing the destructive confirmation's saved plan. */
  async function prepare() {
    if (locked.current || uncertain || project.finalized) return
    locked.current = true
    setPending(true)
    setFailure(undefined)
    try {
      const revision = await flush()
      const saved = await api.refresh()
      if (saved.editPlanRevision !== revision) {
        setFailure({ reason: 'CLIP_FINALIZATION_CONFLICT', params: {} })
        return
      }
      return saved
    } catch (error) {
      setFailure(appFailureFromConnect(error))
    } finally {
      locked.current = false
      setPending(false)
    }
  }
  /** Finalizes as 게시됨 with `publishedUrl`, or as 완료 without one (CLIP-232). */
  async function confirm(publishedUrl?: string) {
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
      await api.finalize({
        expectedRevision: revision,
        expectedResultId: saved.result.id,
        publishedUrl,
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
  /** A finalized clip saves or replaces its URL and nothing else (CLIP-232). */
  async function savePublishedUrl(url: string): Promise<boolean> {
    if (locked.current || !project.finalized) return false
    locked.current = true
    setPending(true)
    setFailure(undefined)
    try {
      await api.savePublishedUrl(url)
      return true
    } catch (error) {
      setFailure(appFailureFromConnect(error))
      return false
    } finally {
      locked.current = false
      setPending(false)
    }
  }

  return {
    prepare,
    confirm,
    checkAgain,
    savePublishedUrl,
    pending,
    uncertain,
    failure,
    busy: pending || uncertain,
  }
}
