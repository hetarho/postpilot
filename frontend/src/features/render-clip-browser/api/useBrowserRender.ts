import { useCallback, useEffect, useRef, useState } from 'react'
import {
  useClipProjectCalls,
  useRefreshClipProjects,
  type ClipNotice,
} from '@/entities/clip-project'
import { useClipPreviewRequest, useClipRenderCalls } from '@/entities/clip-preview'
import { useClipSpeechCalls } from '@/entities/clip-plan'
import { BrowserAudioRenderError } from '../model/audio-preflight'
import { useGenerationJobCalls } from '@/entities/generation-job'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import {
  BrowserRenderSamplingError,
  browserRenderOperations,
  runBrowserRender,
  type BrowserRenderInput,
  type BrowserRenderProgress,
} from './run-render'
import { BrowserRenderVerdictError } from './store-result'
import {
  createBrowserResultStore,
  storeVerifiedBrowserResult,
  type VerifiedBrowserResult,
} from './store-result'
import { BrowserUploadPendingError } from './run-local-render'
import { CLIP_BROWSER_RENDER } from '@/entities/clip-design'
import { reclaimMediaOutputs } from '@/shared/lib/media'

export interface BrowserRenderState {
  phase: 'idle' | 'running' | 'cancelling' | 'cancelled' | 'done' | 'failed' | 'upload_pending'
  progress: BrowserRenderProgress
  failure?: AppFailure
  /** Why the browser kind was refused, beside the reason itself (CLIP-155). */
  refusal?: 'sampling' | 'speech' | 'audio' | 'memory' | 'capability' | 'output'
  notices?: ClipNotice[]
  local?: { url: string; bytes: number; revision: number; ownerId: string; projectId: string }
}
type StartInput = Pick<BrowserRenderInput, 'batchId' | 'localSources' | 'resolvePlayback'> & {
  flush: () => Promise<number>
}

export function useBrowserRender(
  ownerId: string,
  projectId: string,
  planRevision?: number,
  finalized?: boolean,
) {
  const calls = useClipProjectCalls()
  const renders = useClipRenderCalls()
  const speech = useClipSpeechCalls()
  const requestPreview = useClipPreviewRequest()
  const job = useGenerationJobCalls()
  const refresh = useRefreshClipProjects(ownerId)
  const current = useRef<AbortController | undefined>(undefined)
  const provisional = useRef<
    { artifact: VerifiedBrowserResult; url: string; revision: number } | undefined
  >(undefined)
  const mounted = useRef(true)
  const activeRevision = useRef<number | undefined>(undefined)
  const retrying = useRef(false)
  const [state, setState] = useState<BrowserRenderState>({
    phase: 'idle',
    progress: { stage: 'encoding', percent: 0 },
  })
  const visibleState: BrowserRenderState =
    state.local &&
    (!provisional.current ||
      state.local.ownerId !== ownerId ||
      state.local.projectId !== projectId ||
      finalized ||
      (planRevision !== undefined && state.local.revision !== planRevision))
      ? { ...state, phase: 'cancelled', local: undefined }
      : state
  const busy =
    visibleState.phase === 'running' ||
    visibleState.phase === 'cancelling' ||
    visibleState.phase === 'upload_pending'
  const releaseLocal = useCallback(() => {
    const local = provisional.current
    provisional.current = undefined
    if (local) {
      URL.revokeObjectURL(local.url)
      void local.artifact.dispose()
    }
  }, [])
  useEffect(() => {
    mounted.current = true
    void reclaimMediaOutputs(CLIP_BROWSER_RENDER.outputNamespace).catch(() => undefined)
    const leave = () => {
      if (mounted.current && current.current)
        setState((state) => ({ ...state, phase: 'cancelled', local: undefined }))
      current.current?.abort()
      const local = provisional.current
      if (local) void renders.cancelBrowserRender(local.artifact.renderId).catch(() => undefined)
      releaseLocal()
      current.current = undefined
    }
    window.addEventListener('pagehide', leave)
    return () => {
      mounted.current = false
      leave()
      window.removeEventListener('pagehide', leave)
    }
  }, [ownerId, projectId, renders, releaseLocal])
  useEffect(() => {
    if (
      !current.current ||
      (!finalized &&
        (planRevision === undefined ||
          activeRevision.current === undefined ||
          activeRevision.current === planRevision))
    )
      return
    current.current.abort(new DOMException('Render superseded', 'AbortError'))
    const local = provisional.current
    if (local) void renders.cancelBrowserRender(local.artifact.renderId).catch(() => undefined)
    releaseLocal()
    current.current = undefined
  }, [planRevision, finalized, renders, releaseLocal])
  async function start(input: StartInput) {
    if (current.current) return
    const controller = new AbortController()
    current.current = controller
    setState({ phase: 'running', progress: { stage: 'encoding', percent: 0 } })
    let uploadPending = false
    const update = (next: Partial<BrowserRenderState>) => {
      if (mounted.current && current.current === controller)
        setState((state) => ({ ...state, ...next }))
    }
    try {
      const revision = await input.flush()
      activeRevision.current = revision
      controller.signal.throwIfAborted()
      const project = await calls.fetch(projectId, controller.signal)
      if (project.editPlanRevision !== revision || !project.editing) {
        update({ phase: 'failed', failure: { reason: 'CLIP_PLAN_CONFLICT', params: {} } })
        return
      }
      await runBrowserRender(
        {
          ...input,
          loadSpeech: (ref, signal) => speech.load(projectId, ref, signal),
          projectId,
          revision,
          plan: project.editing.plan,
          ratio: project.ratio,
          ownerId,
          onLocalReady: (artifact) => {
            if (controller.signal.aborted || !mounted.current) {
              void artifact.dispose()
              return
            }
            const url = URL.createObjectURL(artifact.file)
            provisional.current = { artifact, url, revision }
            update({ local: { url, bytes: artifact.file.size, revision, ownerId, projectId } })
          },
        },
        browserRenderOperations({
          render: renders,
          fetchProject: calls.fetch,
          requestPreview,
          job,
        }),
        controller.signal,
        (progress) => update({ progress }),
      )
      releaseLocal()
      update({ phase: 'done', local: undefined, progress: { stage: 'storing', percent: 100 } })
    } catch (error) {
      if (error instanceof BrowserUploadPendingError && !controller.signal.aborted) {
        uploadPending = true
        update({ phase: 'upload_pending', failure: appFailureFromConnect(error.cause) })
      } else if (controller.signal.aborted) update({ phase: 'cancelled', local: undefined })
      else if (error instanceof BrowserAudioRenderError)
        update({
          phase: 'failed',
          refusal: error.reason,
          failure: { reason: 'CLIP_PROCESSING_FAILED', params: {} },
        })
      else if (error instanceof BrowserRenderSamplingError)
        // The footage this render is drawn over could not be sampled, so the browser kind is
        // refused with the sampling job's own reason and the server render stays the choice.
        update({
          phase: 'failed',
          refusal: 'sampling',
          failure: error.failure ?? { reason: 'CLIP_PROCESSING_FAILED', params: {} },
        })
      else if (error instanceof BrowserRenderVerdictError)
        update({
          phase: 'failed',
          notices: error.notices.map((n) => ({ ...n, cutId: '', elementId: '' })),
        })
      else if (
        (error instanceof DOMException && error.name === 'QuotaExceededError') ||
        (error instanceof Error &&
          /^(MEDIA_OUTPUT_SIZE_LIMIT|MEDIA_OUTPUT_AUDIO_MEMORY_LIMIT|MEDIA_PACKET_WINDOW_LIMIT)$/u.test(
            error.message,
          ))
      )
        update({
          phase: 'failed',
          refusal: 'output',
          failure: { reason: 'CLIP_PROCESSING_FAILED', params: {} },
        })
      else {
        const failure = appFailureFromConnect(error)
        update({
          phase: 'failed',
          failure:
            failure.reason !== 'UNKNOWN_FAILURE'
              ? failure
              : // A sequence caption the page could not obtain the server's frames
                // for refuses the browser kind by that one reason and leaves the
                // server render as the choice, rather than delivering a caption
                // that stands still (CLIP-155, CLIP-159).
                error instanceof Error && error.message === 'CLIP_CAPTION_FRAMES_UNAVAILABLE'
                ? { reason: 'CLIP_PREVIEW_UNAVAILABLE', params: {} }
                : { reason: 'CLIP_PROCESSING_FAILED', params: {} },
        })
      }
    } finally {
      // Fetch the full server projection, including editing and finalization,
      // rather than replacing the cache with the completion's partial project.
      if (!uploadPending) {
        releaseLocal()
        if (current.current === controller) current.current = undefined
      }
      await refresh.all()
    }
  }
  return {
    state: visibleState,
    busy,
    start,
    retry: async () => {
      const local = provisional.current,
        controller = current.current
      if (!local || !controller || state.phase !== 'upload_pending' || retrying.current) return
      retrying.current = true
      setState((state) => ({
        ...state,
        phase: 'running',
        failure: undefined,
        progress: { stage: 'storing', percent: 80 },
      }))
      try {
        await storeVerifiedBrowserResult(
          local.artifact,
          createBrowserResultStore(renders),
          controller.signal,
          (percent) => {
            if (mounted.current && current.current === controller)
              setState((state) => ({
                ...state,
                progress: { stage: 'storing', percent: 80 + 0.19 * percent },
              }))
          },
        )
        releaseLocal()
        if (mounted.current && current.current === controller)
          setState({ phase: 'done', progress: { stage: 'storing', percent: 100 } })
        if (current.current === controller) current.current = undefined
      } catch (error) {
        if (controller.signal.aborted) {
          releaseLocal()
          if (mounted.current && current.current === controller)
            setState({ phase: 'cancelled', progress: { stage: 'storing', percent: 0 } })
          if (current.current === controller) current.current = undefined
        } else if (mounted.current && current.current === controller)
          setState((state) => ({
            ...state,
            phase: 'upload_pending',
            failure: appFailureFromConnect(error),
          }))
      } finally {
        retrying.current = false
        await refresh.all()
      }
    },
    cancel: () => {
      if (!current.current) return
      setState((state) => ({ ...state, phase: 'cancelling' }))
      current.current.abort()
      if (provisional.current) {
        void renders
          .cancelBrowserRender(provisional.current.artifact.renderId)
          .catch(() => undefined)
        releaseLocal()
        current.current = undefined
        setState({ phase: 'cancelled', progress: { stage: 'storing', percent: 0 } })
        void refresh.all()
      }
    },
  }
}
