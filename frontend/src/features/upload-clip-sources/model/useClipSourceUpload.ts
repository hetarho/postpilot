import { useEffect, useMemo, useSyncExternalStore } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { createClipSourcePipeline } from '../api/pipeline'
import { ClipSourceSession } from './session'

export function useClipSourceUpload(projectId: string) {
  const transport = useTransport()
  const session = useMemo(
    () => new ClipSourceSession(projectId, createClipSourcePipeline(transport)),
    [projectId, transport],
  )
  const state = useSyncExternalStore(session.subscribe, session.getSnapshot)
  useEffect(() => {
    session.activate()
    return () => session.dispose()
  }, [session])
  return {
    ...state,
    select: (files: File[]) => session.select(files),
    cancel: () => session.cancel(),
    finishAttempt: () => session.finishAttempt(),
  }
}
