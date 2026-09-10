import { useEffect, useMemo, useSyncExternalStore } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { createClipSourcePipeline } from '../api/pipeline'
import { ClipSourceSession } from './session'
import type { RetainedClipSource } from '@/entities/clip-project'
import { matchClipSources } from './reselection'

export function useClipSourceUpload(projectId: string, required?: readonly RetainedClipSource[]) {
  const transport = useTransport()
  // A result/plan refetch must not replace the runtime owner of an accepted attempt.
  const session = useMemo(
    () => new ClipSourceSession(projectId, createClipSourcePipeline(transport)),
    [projectId, transport],
  )
  const state = useSyncExternalStore(session.subscribe, session.getSnapshot)
  useEffect(() => {
    session.activate()
    const hide = () => session.dispose()
    const show = () => session.activate()
    window.addEventListener('pagehide', hide)
    window.addEventListener('pageshow', show)
    return () => {
      window.removeEventListener('pagehide', hide)
      window.removeEventListener('pageshow', show)
      session.dispose()
    }
  }, [session])
  const requiredKey = JSON.stringify(required ?? null)
  useEffect(() => session.changeConstraints(requiredKey), [session, requiredKey])
  return {
    ...state,
    select: (files: File[]) =>
      session.select(
        files,
        required
          ? (manifest) => {
              matchClipSources(manifest, required)
            }
          : undefined,
      ),
    cancel: () => session.cancel(),
    beginAttempt: (batchId: string) => session.beginAttempt(batchId),
    markOwned: (batchId: string, jobId: string) => session.markOwned(batchId, jobId),
    rejectAttempt: (batchId: string) => session.rejectAttempt(batchId),
    finishAttempt: (jobId: string, status: 'done' | 'failed') =>
      session.finishAttempt(jobId, status),
  }
}
