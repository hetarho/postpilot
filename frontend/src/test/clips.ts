import { create } from '@bufbuild/protobuf'
import { Code, type createRouterTransport } from '@connectrpc/connect'
import {
  ClipService,
  VideoTemplateSchema,
  CreateVideoTemplateResponseSchema,
  UpdateVideoTemplateResponseSchema,
  DeleteVideoTemplateResponseSchema,
  ListVideoTemplatesResponseSchema,
} from '@/shared/api'
import type { ClipRecipe } from '@/entities/clip-template'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]
export interface FakeClipTemplate extends ClipRecipe {
  id: string
  projectCount?: number
  ownerId?: string
}
export interface FakeClipsOptions {
  templates?: FakeClipTemplate[]
  ownerId?: string
  calls?: string[]
  writes?: ClipRecipe[]
  listFails?: boolean
  saveFails?: boolean
  deleteFails?: boolean
  detachedCount?: number
}
export function registerClipService(router: ConnectRouter, options: FakeClipsOptions = {}) {
  const rows = new Map(
    (options.templates ?? [])
      .filter((r) => !r.ownerId || r.ownerId === options.ownerId)
      .map((r) => [r.id, { ...r }]),
  )
  const toProto = (row: FakeClipTemplate) =>
    create(VideoTemplateSchema, {
      ...row,
      projectCount: row.projectCount ?? 0,
      createdAt: '2026-09-10T00:00:00Z',
      updatedAt: '2026-09-10T00:00:00Z',
    })
  router.rpc(ClipService.method.listVideoTemplates, () => {
    options.calls?.push('ListVideoTemplates')
    if (options.listFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    return create(ListVideoTemplatesResponseSchema, { templates: [...rows.values()].map(toProto) })
  })
  let next = 0
  router.rpc(ClipService.method.createVideoTemplate, (req) => {
    options.calls?.push('CreateVideoTemplate')
    if (options.saveFails) throw connectAppError('CLIP_TEMPLATE_NAME_TAKEN', Code.AlreadyExists)
    const row: FakeClipTemplate = {
      id: `video-template-${++next}`,
      name: req.name.trim(),
      cutGuidance: req.cutGuidance,
      informationFields: req.informationFields.map(({ label, prompt }) => ({ label, prompt })),
      copyStyles: req.copyStyles as ClipRecipe['copyStyles'],
      accent: req.accent as ClipRecipe['accent'],
    }
    options.writes?.push(row)
    rows.set(row.id, row)
    return create(CreateVideoTemplateResponseSchema, { template: toProto(row) })
  })
  router.rpc(ClipService.method.updateVideoTemplate, (req) => {
    options.calls?.push('UpdateVideoTemplate')
    if (options.saveFails) throw connectAppError('CLIP_TEMPLATE_NAME_TAKEN', Code.AlreadyExists)
    const row = rows.get(req.id)
    if (!row) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    if (req.name !== undefined) row.name = req.name
    if (req.cutGuidance !== undefined) row.cutGuidance = req.cutGuidance
    if (req.informationFields)
      row.informationFields = req.informationFields.values.map(({ label, prompt }) => ({
        label,
        prompt,
      }))
    if (req.copyStyles) row.copyStyles = req.copyStyles.values as ClipRecipe['copyStyles']
    if (req.accent !== undefined) row.accent = req.accent as ClipRecipe['accent']
    options.writes?.push({ ...row })
    return create(UpdateVideoTemplateResponseSchema, { template: toProto(row) })
  })
  router.rpc(ClipService.method.deleteVideoTemplate, (req) => {
    options.calls?.push('DeleteVideoTemplate')
    if (options.deleteFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    const row = rows.get(req.id)
    if (!row) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    rows.delete(req.id)
    return create(DeleteVideoTemplateResponseSchema, {
      detachedProjects: options.detachedCount ?? row.projectCount ?? 0,
    })
  })
}
