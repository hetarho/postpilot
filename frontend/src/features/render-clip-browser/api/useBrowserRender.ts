import { useEffect, useRef, useState } from 'react'
import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { clipProjectsKey, toClipProject, type ClipNotice } from '@/entities/clip-project'
import { ClipService, appFailureFromConnect, type AppFailure } from '@/shared/api'
import {
  browserRenderOperations,
  runBrowserRender,
  type BrowserRenderInput,
  type BrowserRenderProgress,
} from './run-render'
import { BrowserRenderVerdictError } from './store-result'

export interface BrowserRenderState {
  phase: 'idle' | 'running' | 'cancelling' | 'cancelled' | 'done' | 'failed'
  progress: BrowserRenderProgress
  failure?: AppFailure
  notices?: ClipNotice[]
}
type StartInput = Pick<BrowserRenderInput, 'batchId' | 'localSources' | 'resolvePlayback'> & {
  flush: () => Promise<number>
}

export function useBrowserRender(ownerId: string, projectId: string) {
  const transport = useTransport(),
    cache = useQueryClient()
  const current = useRef<AbortController | undefined>(undefined)
  const mounted = useRef(true)
  const [state, setState] = useState<BrowserRenderState>({
    phase: 'idle',
    progress: { stage: 'encoding', percent: 0 },
  })
  const busy = state.phase === 'running' || state.phase === 'cancelling'
  useEffect(() => {
    mounted.current = true
    const leave = () => current.current?.abort()
    window.addEventListener('pagehide', leave)
    return () => {
      mounted.current = false
      leave()
      window.removeEventListener('pagehide', leave)
    }
  }, [])
  async function start(input: StartInput) {
    if (current.current) return
    const controller = new AbortController()
    current.current = controller
    setState({ phase: 'running', progress: { stage: 'encoding', percent: 0 } })
    const update = (next: Partial<BrowserRenderState>) => {
      if (mounted.current && current.current === controller)
        setState((state) => ({ ...state, ...next }))
    }
    try {
      const revision = await input.flush()
      controller.signal.throwIfAborted()
      const response = await createClient(ClipService, transport).getClipProject(
        { id: projectId },
        { signal: controller.signal },
      )
      if (!response.project) throw new Error('Missing saved clip')
      const project = toClipProject(response.project)
      if (project.editPlanRevision !== revision || !project.editing) {
        update({ phase: 'failed', failure: { reason: 'CLIP_PLAN_CONFLICT', params: {} } })
        return
      }
      await runBrowserRender(
        { ...input, projectId, revision, plan: project.editing.plan, ratio: project.ratio },
        browserRenderOperations(transport),
        controller.signal,
        (progress) => update({ progress }),
      )
      update({ phase: 'done', progress: { stage: 'storing', percent: 100 } })
    } catch (error) {
      if (controller.signal.aborted) update({ phase: 'cancelled' })
      else if (error instanceof BrowserRenderVerdictError)
        update({
          phase: 'failed',
          notices: error.notices.map((n) => ({ ...n, cutId: '', elementId: '' })),
        })
      else {
        const failure = appFailureFromConnect(error)
        update({
          phase: 'failed',
          failure:
            failure.reason === 'UNKNOWN_FAILURE'
              ? { reason: 'CLIP_PROCESSING_FAILED', params: {} }
              : failure,
        })
      }
    } finally {
      // Fetch the full server projection, including editing and finalization,
      // rather than replacing the cache with the completion's partial project.
      if (current.current === controller) current.current = undefined
      await cache.invalidateQueries({ queryKey: clipProjectsKey(transport, ownerId) })
    }
  }
  return {
    state,
    busy,
    start,
    cancel: () => {
      if (!current.current) return
      setState((state) => ({ ...state, phase: 'cancelling' }))
      current.current.abort()
    },
  }
}
