import { type ClipEditPlan } from '@/entities/clip-plan'
import {
  PreviewAssetCache,
  PreviewPreparation,
  type CaptionFrameLoader,
  type ClipPreviewRequest,
  type PreparedAsset,
} from '@/entities/clip-preview'

/** How the caller asks for one preview request; the entity owns the transport behind it. */
export type PreviewRequestCall = (
  projectId: string,
  revision: number,
  plan: ClipEditPlan,
) => Promise<ClipPreviewRequest>

/** The same server PNGs, manifest checks and runtime-only cache the preview uses. */
export async function prepareBrowserRenderAssets(
  requestPreview: PreviewRequestCall,
  projectId: string,
  revision: number,
  plan: ClipEditPlan,
  signal: AbortSignal,
): Promise<{
  assets: PreparedAsset[]
  width: number
  height: number
  /** The server's own drawing of each frame of a sequence caption, pinned to this
   *  request's plan and hash (CLIP-159). */
  captionFrames: CaptionFrameLoader
  dispose: () => void
}> {
  signal.throwIfAborted()
  const request = await requestPreview(projectId, revision, plan)
  signal.throwIfAborted()
  const preparation = new PreviewPreparation(
    new PreviewAssetCache({
      create: (png) => URL.createObjectURL(new Blob([new Uint8Array(png)], { type: 'image/png' })),
      revoke: URL.revokeObjectURL,
    }),
  )
  try {
    return await new Promise((resolve, reject) => {
      const stop = () => {
        unsubscribe()
        signal.removeEventListener('abort', aborted)
      }
      const aborted = () => {
        stop()
        reject(signal.reason)
      }
      const unsubscribe = preparation.subscribe(() => {
        const snapshot = preparation.getSnapshot()
        if (snapshot.status === 'failed') {
          stop()
          reject(snapshot.error)
        }
        if (snapshot.status === 'ready') {
          stop()
          resolve({
            assets: snapshot.assets,
            width: snapshot.canvasWidth,
            height: snapshot.canvasHeight,
            captionFrames: request.frames,
            dispose: () => preparation.dispose(),
          })
        }
      })
      signal.addEventListener('abort', aborted, { once: true })
      preparation.update(request.hash, request.hash, [], request.load)
    })
  } catch (error) {
    preparation.dispose()
    throw error
  }
}
