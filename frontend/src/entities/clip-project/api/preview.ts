import { create, toBinary, toJsonString } from '@bufbuild/protobuf'
import { createClient, type Transport } from '@connectrpc/connect'
import {
  ClipEditPlanSchema,
  ClipPreviewParity,
  ClipService,
  PrepareClipPreviewRequestSchema,
} from '@/shared/api'
import { CLIP_DRAFT_PREVIEW } from '@/shared/config'
import type { ClipEditPlan } from '../model/edit-plan'
import type { PreviewPage } from '../model/draft-preview'
import { clipPlanToProto } from './edit-plan'

export async function clipPreviewRequest(
  transport: Transport,
  projectId: string,
  revision: number,
  draft: ClipEditPlan,
) {
  const plan = create(ClipEditPlanSchema, clipPlanToProto(draft))
  const bytes = toBinary(ClipEditPlanSchema, plan)
  if (bytes.byteLength > CLIP_DRAFT_PREVIEW.maxRequestBytes)
    throw new Error('CLIP_PREVIEW_TOO_LARGE')
  const digest = new Uint8Array(await crypto.subtle.digest('SHA-256', new Uint8Array(bytes)))
  const hash = [...digest].map((v) => v.toString(16).padStart(2, '0')).join('')
  return {
    hash,
    async load(
      elementIds: string[],
      assetOffset: number,
      signal: AbortSignal,
    ): Promise<PreviewPage> {
      const request = create(PrepareClipPreviewRequestSchema, {
        projectId,
        expectedRevision: revision,
        draftHash: hash,
        plan,
        elementIds,
        assetOffset,
      })
      if (
        new TextEncoder().encode(toJsonString(PrepareClipPreviewRequestSchema, request))
          .byteLength > CLIP_DRAFT_PREVIEW.maxRequestBytes
      )
        throw new Error('CLIP_PREVIEW_TOO_LARGE')
      const value = await createClient(ClipService, transport).prepareClipPreview(request, {
        signal,
      })
      return {
        draftHash: value.draftHash,
        canvasWidth: value.canvasWidth,
        canvasHeight: value.canvasHeight,
        nextOffset: value.nextOffset,
        assets: value.assets.map(({ $typeName, ...asset }) => {
          void $typeName
          return asset
        }),
        parity: value.parity.map((p) => {
          switch (p) {
            case ClipPreviewParity.SOURCE_CONTRAST_FINAL_ONLY:
              return 'sourceContrast'
            case ClipPreviewParity.AUDIO_NORMALIZATION_FINAL_ONLY:
              return 'audioNormalization'
            case ClipPreviewParity.BROWSER_FRAME_TIMING:
              return 'frameTiming'
            default:
              return 'unknown'
          }
        }),
      }
    },
  }
}
