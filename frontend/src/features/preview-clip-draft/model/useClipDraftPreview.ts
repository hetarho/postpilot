import { useCallback, useEffect, useMemo, useState, useSyncExternalStore } from 'react'
import { appFailureFromConnect, normalizeAppFailure } from '@/shared/api'
import type { ClipEditPlan } from '@/entities/clip-plan'
import {
  PreviewAssetCache,
  PreviewPreparation,
  previewElementIDs,
  previewTimeline,
  useClipPreviewRequest,
  type ClipPreviewOverlay,
  type ClipPreviewRequest,
} from '@/entities/clip-preview'

interface PreviewRequestState {
  /** The (project, revision, plan, retry) this answer belongs to; a pending state carries none. */
  token: string
  request?: ClipPreviewRequest
  error?: unknown
}

/** The transport half. One request per (project, revision, plan): the element ids the preview
 *  later asks for follow the play position and must NOT re-issue it. While a new plan's request
 *  is in flight the state is pending, so the preparation is never handed a stale draft hash. */
export function usePreviewRequest(
  projectId: string,
  revision: number,
  planJSON: string,
  retry: number,
): PreviewRequestState {
  const request = useClipPreviewRequest()
  const token = JSON.stringify([projectId, revision, planJSON, retry])
  const [state, setState] = useState<PreviewRequestState>({ token: '' })
  useEffect(() => {
    let active = true
    void request(projectId, revision, JSON.parse(planJSON) as ClipEditPlan).then(
      (value) => {
        if (active) setState({ token, request: value })
      },
      (error: unknown) => {
        if (active) setState({ token, error })
      },
    )
    return () => {
      active = false
    }
  }, [request, projectId, revision, planJSON, token])
  return state.token === token ? state : { token: '' }
}

/** The assets half. Owns the per-mount preparation (and the object URLs it hands out), and asks
 *  it for exactly the elements on screen whenever the position or the answered request changes. */
export function usePreviewPreparation(
  key: string,
  contentKey: string,
  idsJSON: string,
  result: PreviewRequestState,
) {
  const [preparation] = useState(
    () =>
      new PreviewPreparation(
        new PreviewAssetCache({
          create: (bytes) =>
            URL.createObjectURL(new Blob([new Uint8Array(bytes)], { type: 'image/png' })),
          revoke: (url) => URL.revokeObjectURL(url),
        }),
      ),
  )
  const snapshot = useSyncExternalStore(preparation.subscribe, preparation.getSnapshot)
  useEffect(() => {
    if (result.request)
      preparation.update(
        key,
        result.request.hash,
        JSON.parse(idsJSON) as string[],
        result.request.load,
        contentKey,
      )
    else if (result.token)
      preparation.update(key, '', [], () => Promise.reject(result.error), contentKey)
    return () => {
      preparation.stop()
    }
  }, [preparation, result, key, contentKey, idsJSON])
  useEffect(
    () => () => {
      preparation.dispose()
    },
    [preparation],
  )
  return snapshot
}

/** Everything `ClipDraftPreview` needs but may not do itself (ARCH-14): the preview request for
 *  the plan being edited, the overlay prepared for the position on screen, and the way back from
 *  a failure. The entity component renders the result and owns no transport. */
export function useClipDraftPreview({
  projectId,
  revision,
  plan,
  timeMs,
}: {
  projectId: string
  revision: number
  plan: ClipEditPlan
  timeMs: number
}): ClipPreviewOverlay {
  const [retry, setRetry] = useState(0)
  const planJSON = JSON.stringify(plan)
  const timeline = useMemo(() => previewTimeline(JSON.parse(planJSON) as ClipEditPlan), [planJSON])
  const idsJSON = JSON.stringify(previewElementIDs(plan, timeline, timeMs))
  const contentKey = JSON.stringify([projectId, revision, planJSON])
  const key = JSON.stringify([contentKey, idsJSON, retry])
  const result = usePreviewRequest(projectId, revision, planJSON, retry)
  const snapshot = usePreviewPreparation(key, contentKey, idsJSON, result)
  const failed = snapshot.key === key && snapshot.status === 'failed'
  return {
    assets: snapshot.assets,
    canvasWidth: snapshot.canvasWidth,
    canvasHeight: snapshot.canvasHeight,
    ready:
      snapshot.contentKey === contentKey &&
      snapshot.canvasWidth > 0 &&
      snapshot.status !== 'failed',
    updating: snapshot.status === 'updating',
    // A refusal the browser never sent is ours, not the server's: it carries no Connect code.
    failure: failed
      ? snapshot.error instanceof Error && snapshot.error.message === 'CLIP_PREVIEW_TOO_LARGE'
        ? normalizeAppFailure({ reason: snapshot.error.message, params: {} })
        : appFailureFromConnect(snapshot.error)
      : undefined,
    onRetry: useCallback(() => setRetry((value) => value + 1), []),
  }
}
