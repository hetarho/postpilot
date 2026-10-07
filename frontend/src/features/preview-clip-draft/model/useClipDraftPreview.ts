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
/** Page ownership around the shared local engine. Scrubbing never requests a server raster. */
export function useClipDraftPreview(
  input: ClipDraftPreviewInput,
): ClipPreviewOverlay & { local?: ClipLocalCompositionRuntime } {
  const [retry, setRetry] = useState(0)
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
      source.url,
      !!source.file,
    ]),
    retry,
  ])
  const [state, setState] = useState<{
    key: string
    local?: ClipLocalCompositionRuntime
    error?: unknown
  }>({ key: '' })
  const latest = useRef(input)
  useLayoutEffect(() => {
    latest.current = input
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
          (fingerprint, refresh) => data.resolvePlayback(fingerprint, refresh || retry > 0),
          controller.signal,
        )
        setState({
          key,
          local: { snapshot, source: originals.source, audioSource: originals.audioSource },
        })
      },
      (error: unknown) => {
        if (!controller.signal.aborted) setState({ key, error })
      },
    )
    return () => {
      controller.abort()
      originals?.dispose()
    }
    // `key` is the detached content/runtime identity; output playhead changes preserve it.
  }, [key, retry])
  const current = state.key === key ? state : undefined,
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
    onRetry: useCallback(() => setRetry((value) => value + 1), []),
  }
}
