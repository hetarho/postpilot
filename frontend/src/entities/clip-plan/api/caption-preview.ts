import { create } from '@bufbuild/protobuf'
import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import {
  ClipEditPlanSchema,
  ClipPlanService,
  GetClipCaptionPreviewRequestSchema,
  GetClipCaptionStyleSamplesRequestSchema,
  type ProtoGetClipCaptionPreviewResponse,
} from '@/shared/api'
import type { ClipEditPlan } from '../model/edit-plan'
import { clipPlanToProto } from './edit-plan'

export interface ClipCanvasBox {
  x: number
  y: number
  width: number
  height: number
}
/** One caption as the RENDERER draws it, brought to the origin (CDS-83). The
 *  `svg` is placed by translating it to `box`, so a drag never asks the server
 *  anything, and the two transforms cancel exactly. */
export interface ClipCaptionFragment {
  instanceId: string
  svg: string
  box: ClipCanvasBox
  fontSize: number
  style: string
  /** A sequence-rendered style draws one layer per output frame; this is one
   *  representative frame of that motion, not the motion itself (CDS-81). */
  representativeFrame: boolean
}
export interface ClipCaptionPreview {
  ratio: string
  canvas: ClipCanvasBox
  /** The design safe area every caption is clamped inside. It comes from the
   *  server so the editor mirrors no CDS-9/CDS-13 geometry of its own. */
  safeArea: ClipCanvasBox
  captions: ClipCaptionFragment[]
}

const box = (value?: ClipCanvasBox): ClipCanvasBox => ({
  x: value?.x ?? 0,
  y: value?.y ?? 0,
  width: value?.width ?? 0,
  height: value?.height ?? 0,
})

export function toClipCaptionPreview(
  value: ProtoGetClipCaptionPreviewResponse,
): ClipCaptionPreview {
  return {
    ratio: value.ratio,
    canvas: box(value.canvas),
    safeArea: box(value.safeArea),
    captions: value.captions.map((c) => ({
      instanceId: c.instanceId,
      svg: c.svg,
      box: box(c.box),
      fontSize: c.fontSize,
      style: c.style,
      representativeFrame: c.representativeFrame,
    })),
  }
}

/** What the captions of a plan actually DRAW. The query is keyed by it, so
 *  moving a caption or selecting another one asks the server nothing: only the
 *  words, the style, the size and the pacing change what is drawn. */
export function captionDrawingKey(plan: ClipEditPlan) {
  return (plan.elements ?? [])
    .filter((text) => text.role === 'caption')
    .map((text) =>
      [
        text.instanceId,
        text.text,
        text.ownerStyle ?? '',
        text.ownerSizePx ?? 0,
        text.pace,
        text.accent,
        text.keyword,
      ].join('~'),
    )
    .join('|')
}

export function useClipCaptionPreview(
  projectId: string | undefined,
  revision: number,
  plan: ClipEditPlan,
  enabled: boolean,
) {
  const transport = useTransport()
  return useQuery<ClipCaptionPreview>({
    queryKey: ['clip-caption-preview', transport, projectId, revision, captionDrawingKey(plan)],
    enabled: enabled && !!projectId && revision > 0,
    staleTime: Infinity,
    retry: false,
    queryFn: async ({ signal }) => {
      const value = await createClient(ClipPlanService, transport).getClipCaptionPreview(
        create(GetClipCaptionPreviewRequestSchema, {
          projectId,
          expectedRevision: revision,
          plan: create(ClipEditPlanSchema, clipPlanToProto(plan)),
        }),
        { signal },
      )
      return toClipCaptionPreview(value)
    },
  })
}

/** Every approved caption style, drawn once by the renderer itself (CDS-83), so
 *  ① offers the set by its own look rather than by a picture of it. It asks the
 *  project for nothing but its ratio, so the answer is the same for every
 *  project of that ratio and is cached for the session. */
export function useClipCaptionStyleSamples(projectId: string | undefined, enabled: boolean) {
  const transport = useTransport()
  return useQuery<ClipCaptionPreview>({
    queryKey: ['clip-caption-style-samples', transport, projectId],
    enabled: enabled && !!projectId,
    staleTime: Infinity,
    retry: false,
    queryFn: async ({ signal }) => {
      const value = await createClient(ClipPlanService, transport).getClipCaptionStyleSamples(
        create(GetClipCaptionStyleSamplesRequestSchema, { projectId }),
        { signal },
      )
      return {
        ratio: value.ratio,
        canvas: box(value.canvas),
        safeArea: box(undefined),
        captions: value.samples.map((c) => ({
          instanceId: c.instanceId,
          svg: c.svg,
          box: box(c.box),
          fontSize: c.fontSize,
          style: c.style,
          representativeFrame: c.representativeFrame,
        })),
      }
    },
  })
}
