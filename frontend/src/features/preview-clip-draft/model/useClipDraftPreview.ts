import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { normalizeAppFailure } from '@/shared/api'
import type { ClipEditPlan, RetainedClipSource } from '@/entities/clip-plan'
import {
  BrowserCompositionOriginals,
  freezeBrowserPreviewComposition,
  projectBrowserComposition,
  type BrowserCompositionDesign,
  type BrowserLocalOriginal,
  type ClipLocalCompositionRuntime,
  type ClipPreviewOverlay,
} from '@/entities/clip-preview'
import type { ClipLayoutObservations } from '@/entities/clip-observation'
import { CLIP_DESIGN, type ClipRatioId } from '@/entities/clip-design'

export interface ClipDraftPreviewInput {
  ownerId: string
  projectId: string
  projectRevision?: number
  revision: number
  plan: ClipEditPlan
  ratio: ClipRatioId
  sources: readonly RetainedClipSource[]
  layoutObservations?: ClipLayoutObservations
  design?: Partial<BrowserCompositionDesign>
  localSources?: readonly BrowserLocalOriginal[]
  resolvePlayback: (fingerprint: string, refresh?: boolean) => Promise<string>
  timeMs?: number
}

function localAccessIdentity(source: BrowserLocalOriginal) {
  const hint = source.retainedAccess
  if (
    !source.file &&
    hint &&
    typeof hint.state === 'string' &&
    Number.isSafeInteger(hint.generation) &&
    hint.generation >= 0
  ) {
    const url = source.url ?? ''
    let http = false
    try {
      http = ['http:', 'https:'].includes(new URL(url, 'https://retained.invalid').protocol)
    } catch {
      /* Unknown access shapes keep the raw URL fence. */
    }
    if ((!url && hint.generation !== 1) || (http && url && hint.generation > 0))
      return ['retained', Math.max(0, hint.generation - 1), hint.state]
  }
  return source.url
}

const retryScope = (data: ClipDraftPreviewInput) =>
  JSON.stringify([
    data.ownerId,
    data.projectId,
    data.projectRevision ?? data.revision,
    data.revision,
    data.plan.cuts.map((cut) => [cut.sourceId, cut.fingerprint]),
  ])
const retrySources = (data: ClipDraftPreviewInput) =>
  data.plan.cuts.map((cut) => {
    const local = data.localSources?.find(
      (source) => source.sourceId === cut.sourceId && source.fingerprint === cut.fingerprint,
    )
    return {
      sourceId: cut.sourceId,
      fingerprint: cut.fingerprint,
      file: local?.file,
      present: !!local,
      blob: local?.url?.startsWith('blob:') ? local.url : undefined,
    }
  })
type RetryRequest =
  { status: 'succeeded' } | { status: 'pending' | 'failed'; promise: Promise<string> }
interface ExplicitRetry {
  scope: string
  resolver: ClipDraftPreviewInput['resolvePlayback']
  sources: ReturnType<typeof retrySources>
  requests: Map<string, RetryRequest>
}
function matchesRetry(attempt: ExplicitRetry, data: ClipDraftPreviewInput) {
  const sources = retrySources(data)
  return (
    attempt.scope === retryScope(data) &&
    attempt.resolver === data.resolvePlayback &&
    attempt.sources.every(
      (source, index) =>
        source.file === sources[index]?.file &&
        source.blob === sources[index]?.blob &&
        source.present === sources[index]?.present,
    )
  )
}
/** Page ownership around the shared local engine. Scrubbing never requests a server raster. */
export function useClipDraftPreview(
  input: ClipDraftPreviewInput,
): ClipPreviewOverlay & { local?: ClipLocalCompositionRuntime } {
  const [retry, setRetry] = useState(0)
  const explicitRetry = useRef<ExplicitRetry | undefined>(undefined)
  const nextFiles = (input.localSources ?? []).map((source) => source.file)
  const [files, setFiles] = useState(nextFiles)
  if (files.length !== nextFiles.length || files.some((file, index) => file !== nextFiles[index]))
    setFiles(nextFiles)
  const key = JSON.stringify([
    input.ownerId,
    input.projectId,
    input.projectRevision ?? input.revision,
    input.revision,
    input.plan,
    input.ratio,
    input.sources,
    input.layoutObservations,
    input.design,
    (input.localSources ?? []).map((source) => [
      source.sourceId,
      source.fingerprint,
      localAccessIdentity(source),
      !!source.file,
    ]),
    retry,
  ])
  const [state, setState] = useState<{
    key: string
    local?: ClipLocalCompositionRuntime
    error?: unknown
    files?: typeof files
    resolver?: ClipDraftPreviewInput['resolvePlayback']
  }>({ key: '' })
  const latest = useRef(input)
  useLayoutEffect(() => {
    latest.current = input
    if (explicitRetry.current && !matchesRetry(explicitRetry.current, input))
      explicitRetry.current = undefined
  }, [input])
  useEffect(() => {
    const data = latest.current
    const controller = new AbortController()
    let originals: BrowserCompositionOriginals | undefined
    void freezeBrowserPreviewComposition(
      projectBrowserComposition({
        ownerId: data.ownerId,
        projectId: data.projectId,
        projectRevision: data.projectRevision ?? data.revision,
        planRevision: data.revision,
        plan: data.plan,
        ratio: data.ratio,
        sources: data.sources,
        layoutObservations: data.layoutObservations,
        design: data.design,
      }),
    ).then(
      (snapshot) => {
        if (controller.signal.aborted) return
        originals = new BrowserCompositionOriginals(
          snapshot,
          data.localSources ?? [],
          (fingerprint, refresh = false) => {
            if (refresh) return data.resolvePlayback(fingerprint, true)
            const attempt = explicitRetry.current
            const used = attempt?.sources.find((source) => source.fingerprint === fingerprint)
            if (!attempt || !used || !matchesRetry(attempt, data))
              return data.resolvePlayback(fingerprint, false)
            const id = JSON.stringify([used.sourceId, fingerprint])
            const previous = attempt.requests.get(id)
            if (previous?.status === 'succeeded') return data.resolvePlayback(fingerprint, false)
            if (previous) return previous.promise
            // Consume before invoking the resolver: synchronous publication or
            // another owner shares pending work and cannot repeat a force.
            const request: RetryRequest = {
              status: 'pending',
              promise: Promise.resolve().then(() => data.resolvePlayback(fingerprint, true)),
            }
            attempt.requests.set(id, request)
            void request.promise.then(
              () => {
                attempt.requests.set(id, { status: 'succeeded' })
              },
              () => {
                request.status = 'failed'
              },
            )
            return request.promise
          },
          controller.signal,
        )
        setState({
          key,
          files,
          resolver: data.resolvePlayback,
          local: { snapshot, source: originals.source, audioSource: originals.audioSource },
        })
      },
      (error: unknown) => {
        if (!controller.signal.aborted)
          setState({ key, files, resolver: data.resolvePlayback, error })
      },
    )
    return () => {
      controller.abort()
      originals?.dispose()
    }
    // `key` is the detached content/runtime identity; output playhead changes preserve it.
  }, [key, retry, files, input.resolvePlayback])
  const current =
      state.key === key && state.files === files && state.resolver === input.resolvePlayback
        ? state
        : undefined,
    canvas = CLIP_DESIGN.ratios[input.ratio].canvas
  return {
    local: current?.local,
    assets: [],
    canvasWidth: canvas.width,
    canvasHeight: canvas.height,
    ready: !!current?.local,
    updating: !current,
    failure: current?.error
      ? normalizeAppFailure({
          reason:
            current.error instanceof Error
              ? current.error.message.split(':')[0]!
              : 'CLIP_PREVIEW_UNAVAILABLE',
          params: {},
        })
      : undefined,
    onRetry: useCallback(() => {
      const data = latest.current
      explicitRetry.current = {
        scope: retryScope(data),
        resolver: data.resolvePlayback,
        sources: retrySources(data),
        requests: new Map(),
      }
      setRetry((value) => value + 1)
    }, []),
  }
}
