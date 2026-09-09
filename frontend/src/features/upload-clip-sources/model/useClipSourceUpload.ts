import { useEffect, useMemo, useSyncExternalStore } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { createClipSourcePipeline } from '../api/pipeline'
import { ClipSourceSession } from './session'
import type { RetainedClipSource } from '@/entities/clip-project'
import { matchClipSources } from './reselection'

export function useClipSourceUpload(projectId: string, required?: readonly RetainedClipSource[]) {
  const transport = useTransport()
  // Plan field edits must not dispose already selected files when the required
  // source metadata is unchanged. Only this byte-free projection is serialized.
  const requiredJSON = JSON.stringify(required)
  const stableRequired = useMemo(
    () =>
      requiredJSON === undefined ? undefined : (JSON.parse(requiredJSON) as RetainedClipSource[]),
    [requiredJSON],
  )
  const session = useMemo(() => {
    const pipeline = createClipSourcePipeline(transport)
    const read = pipeline.read
    return new ClipSourceSession(projectId, {
      ...pipeline,
      read: async (files, signal) => {
        const manifest = await read(files, signal)
        if (stableRequired) matchClipSources(manifest, stableRequired)
        return manifest
      },
    })
  }, [projectId, transport, stableRequired])
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
