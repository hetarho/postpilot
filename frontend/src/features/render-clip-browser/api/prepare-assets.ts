import type { Transport } from '@connectrpc/connect'
import { type ClipEditPlan } from '@/entities/clip-plan'
import {
  PreviewAssetCache,
  PreviewPreparation,
  clipPreviewRequest,
  type PreparedAsset,
} from '@/entities/clip-preview'

/** The same server PNGs, manifest checks and runtime-only cache the preview uses. */
export async function prepareBrowserRenderAssets(
  transport: Transport,
  projectId: string,
  revision: number,
  plan: ClipEditPlan,
  signal: AbortSignal,
): Promise<{ assets: PreparedAsset[]; width: number; height: number; dispose: () => void }> {
  signal.throwIfAborted()
  const request = await clipPreviewRequest(transport, projectId, revision, plan)
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
