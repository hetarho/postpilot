import { useEffect, useLayoutEffect, useMemo, useSyncExternalStore } from 'react'
import { createClipSourcePipeline } from '../api/pipeline'
import { ClipSourceSession } from './session'
import type { RetainedClipSource } from '@/entities/clip-plan'
import { useClipSourceCalls } from '@/entities/clip-project'
import { POLL_INTERVAL_MS } from '@/shared/config'
import { matchClipSources } from './reselection'
import { claimClipSourcePick } from './pick-handoff'

export function useClipSourceUpload(
  projectId: string,
  required?: readonly RetainedClipSource[],
  enabled = true,
) {
  const calls = useClipSourceCalls()
  // A result/plan refetch must not replace the runtime owner of an accepted attempt.
  const session = useMemo(
    () => new ClipSourceSession(projectId, createClipSourcePipeline(calls)),
    [projectId, calls],
  )
  const state = useSyncExternalStore(session.subscribe, session.getSnapshot)
  // Own the runtime before child preview effects request retained playback.
  useLayoutEffect(() => {
    if (!enabled) {
      session.dispose()
      return
    }
    session.activate()
    // A pick made on the unminted `/clips/new` uploads here, to the id it minted (CLIP-130).
    // Claimed a task later, so Strict Mode's discarded first activation does not take it.
    const handed = window.setTimeout(() => {
      const files = claimClipSourcePick(projectId)
      if (files?.length) void session.select(files)
    }, 0)
    const hide = () => session.dispose()
    const show = () => session.activate()
    window.addEventListener('pagehide', hide)
    window.addEventListener('pageshow', show)
    return () => {
      window.clearTimeout(handed)
      window.removeEventListener('pagehide', hide)
      window.removeEventListener('pageshow', show)
      session.dispose()
    }
  }, [session, enabled, projectId])
  const requiredKey = JSON.stringify(required ?? null)
  useEffect(() => {
    const sources = JSON.parse(requiredKey) as RetainedClipSource[] | null
    session.requireSources(sources?.map((s) => s.fingerprint))
  }, [session, requiredKey])
  useEffect(() => {
    if (state.phase !== 'finished') return
    const timer = window.setInterval(() => void session.refreshRetained(), POLL_INTERVAL_MS)
    return () => window.clearInterval(timer)
  }, [session, state.phase])
  useEffect(() => {
    const at = state.readyBatch?.expiresAt
    if (!at) return
    const delay = Date.parse(at) - Date.now()
    if (!Number.isFinite(delay) || delay > 2 ** 31 - 1) return
    const timer = window.setTimeout(() => void session.refreshRetained(), Math.max(0, delay))
    return () => window.clearTimeout(timer)
  }, [session, state.readyBatch?.expiresAt])
  return {
    ...state,
    ensurePlayback: session.ensurePlayback,
    acceptSoundBatch: session.acceptSoundBatch,
    acceptSourceOrder: session.acceptSourceOrder,
    refreshRetained: session.refreshRetained,
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
    finishAttempt: (jobId: string, status: 'done' | 'failed' | 'cancelled') =>
      session.finishAttempt(jobId, status),
  }
}
