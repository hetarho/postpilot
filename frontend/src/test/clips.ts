import { create } from '@bufbuild/protobuf'
import { Code, type createRouterTransport } from '@connectrpc/connect'
import {
  ClipService,
  VideoTemplateSchema,
  CreateVideoTemplateResponseSchema,
  UpdateVideoTemplateResponseSchema,
  DeleteVideoTemplateResponseSchema,
  ListVideoTemplatesResponseSchema,
  ClipProjectSchema,
  ClipSourceBatchSchema,
  ListClipProjectsResponseSchema,
  GetClipProjectResponseSchema,
  CreateClipProjectResponseSchema,
  UpdateClipProjectResponseSchema,
  DeleteClipProjectResponseSchema,
  CreateClipSourceBatchResponseSchema,
  ConfirmClipSourceResponseSchema,
  DiscardClipSourceBatchResponseSchema,
  StartClipGenerationResponseSchema,
  SaveClipEditPlanResponseSchema,
  StartClipRenderResponseSchema,
  ClipEditingStateSchema,
  type ProtoClipSourceBatch,
} from '@/shared/api'
import type { ClipRecipe } from '@/entities/clip-template'
import {
  toClipEditingState,
  type ClipProject,
  type ClipProjectDraft,
  type ClipEditPlan,
} from '@/entities/clip-project'
import { toFakeProto, type FakeGenerationJobRow } from './jobs'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]
export interface FakeClipTemplate extends ClipRecipe {
  id: string
  projectCount?: number
  ownerId?: string
}
export interface FakeClipProject extends ClipProjectDraft {
  id: string
  ownerId?: string
  result?: ClipProject['result']
  latestJob?: FakeGenerationJobRow
  editing?: ClipProject['editing']
  editPlanRevision?: number
  renderedPlanRevision?: number
}
export interface FakeClipsOptions {
  projects?: FakeClipProject[]
  readProject?: (project: FakeClipProject) => FakeClipProject
  generationStarts?: unknown[]
  generationJobId?: string
  generationFails?: boolean
  renderStarts?: unknown[]
  renderJobId?: string
  renderFails?: boolean
  planWrites?: Array<{ revision: number; plan: ClipEditPlan }>
  planSaveConflict?: boolean
  projectWrites?: ClipProjectDraft[]
  projectSaveFails?: boolean
  projectListFails?: boolean
  sourceRequests?: unknown[]
  reserveFails?: boolean
  confirmFails?: boolean
  discardFails?: boolean
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
  const projects = new Map<string, FakeClipProject>(
    (options.projects ?? [])
      .filter((p) => !p.ownerId || p.ownerId === options.ownerId)
      .map((p) => [p.id, { ...p, answers: p.answers.map((a) => ({ ...a })) }]),
  )
  const projectProto = (p: FakeClipProject) =>
    create(ClipProjectSchema, {
      ...p,
      result: p.result ? { ...p.result, bytes: BigInt(p.result.bytes) } : undefined,
      latestJob: p.latestJob ? toFakeProto(p.latestJob) : undefined,
      createdAt: '2026-09-10T00:00:00Z',
      updatedAt: '2026-09-10T00:00:00Z',
    })
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
    for (const p of projects.values()) if (p.videoTemplateId === req.id) p.videoTemplateId = ''
    return create(DeleteVideoTemplateResponseSchema, {
      detachedProjects: options.detachedCount ?? row.projectCount ?? 0,
    })
  })
  router.rpc(ClipService.method.listClipProjects, () => {
    options.calls?.push('ListClipProjects')
    if (options.projectListFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    return create(ListClipProjectsResponseSchema, {
      projects: [...projects.values()].map(projectProto),
    })
  })
  router.rpc(ClipService.method.getClipProject, (req) => {
    options.calls?.push('GetClipProject')
    const p = projects.get(req.id)
    if (!p) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    return create(GetClipProjectResponseSchema, {
      project: projectProto(options.readProject?.(p) ?? p),
    })
  })
  router.rpc(ClipService.method.createClipProject, (req) => {
    options.calls?.push('CreateClipProject')
    if (options.projectSaveFails) throw connectAppError('CLIP_INVALID_INPUT', Code.InvalidArgument)
    const p = {
      id: `clip-${++next}`,
      title: req.title,
      videoTemplateId: req.videoTemplateId,
      ratio: req.ratio as ClipProjectDraft['ratio'],
      targetDurationMs: req.targetDurationMs,
      answers: req.answers.map((a) => ({ label: a.label, text: a.text })),
    }
    projects.set(p.id, p)
    options.projectWrites?.push(p)
    return create(CreateClipProjectResponseSchema, { project: projectProto(p) })
  })
  router.rpc(ClipService.method.updateClipProject, (req) => {
    options.calls?.push('UpdateClipProject')
    if (options.projectSaveFails) throw connectAppError('CLIP_INVALID_INPUT', Code.InvalidArgument)
    const p = projects.get(req.id)
    if (!p) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    if (req.title !== undefined) p.title = req.title
    if (req.videoTemplateId !== undefined) p.videoTemplateId = req.videoTemplateId
    if (req.targetDurationMs !== undefined) p.targetDurationMs = req.targetDurationMs
    for (const answer of req.answers)
      p.answers = [
        ...p.answers.filter((a) => a.label !== answer.label),
        { label: answer.label, text: answer.text },
      ]
    options.projectWrites?.push({ ...p })
    return create(UpdateClipProjectResponseSchema, { project: projectProto(p) })
  })
  router.rpc(ClipService.method.deleteClipProject, (req) => {
    options.calls?.push('DeleteClipProject')
    if (options.deleteFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    if (!projects.delete(req.id)) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    return create(DeleteClipProjectResponseSchema, {})
  })
  const batches = new Map<string, ProtoClipSourceBatch>()
  router.rpc(ClipService.method.saveClipEditPlan, (req) => {
    options.calls?.push('SaveClipEditPlan')
    const p = projects.get(req.projectId)
    if (!p?.editing) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    if (options.planSaveConflict || req.expectedRevision !== p.editPlanRevision)
      throw connectAppError('CLIP_PLAN_CONFLICT', Code.Aborted)
    p.editing = toClipEditingState(create(ClipEditingStateSchema, { ...p.editing, plan: req.plan }))
    options.planWrites?.push({ revision: req.expectedRevision, plan: p.editing.plan })
    p.editPlanRevision = (p.editPlanRevision ?? 0) + 1
    return create(SaveClipEditPlanResponseSchema, { project: projectProto(p) })
  })
  router.rpc(ClipService.method.startClipRender, (req) => {
    options.calls?.push('StartClipRender')
    options.renderStarts?.push(req)
    if (options.renderFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    const p = projects.get(req.projectId),
      b = batches.get(req.batchId)
    if (!p || !b || b.state !== 'ready')
      throw connectAppError('CLIP_SOURCE_UNAVAILABLE', Code.FailedPrecondition)
    if (p.editPlanRevision !== req.expectedRevision)
      throw connectAppError('CLIP_PLAN_CONFLICT', Code.Aborted)
    b.state = 'consuming'
    const jobId = options.renderJobId ?? 'clip-render-job'
    p.latestJob = {
      id: jobId,
      kind: 'render_clip',
      status: 'queued',
      stage: 'prepare',
      clipProjectId: p.id,
    }
    return create(StartClipRenderResponseSchema, { jobId })
  })
  router.rpc(ClipService.method.startClipGeneration, (req) => {
    options.calls?.push('StartClipGeneration')
    options.generationStarts?.push(req)
    if (options.generationFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    const p = projects.get(req.projectId)
    const b = batches.get(req.batchId)
    if (!p || !b || b.state !== 'ready')
      throw connectAppError('CLIP_SOURCE_UNAVAILABLE', Code.FailedPrecondition)
    b.state = 'consuming'
    const jobId = options.generationJobId ?? 'clip-job'
    p.latestJob = {
      id: jobId,
      kind: 'generate_clip',
      status: 'queued',
      stage: 'prepare',
      clipProjectId: p.id,
    }
    return create(StartClipGenerationResponseSchema, { jobId })
  })
  router.rpc(ClipService.method.createClipSourceBatch, (req) => {
    options.calls?.push('CreateClipSourceBatch')
    options.sourceRequests?.push(req)
    if (options.reserveFails)
      throw connectAppError('CLIP_SOURCE_UNAVAILABLE', Code.FailedPrecondition)
    if (!projects.has(req.projectId)) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    const id = `batch-${++next}`
    const batch = create(ClipSourceBatchSchema, {
      id,
      projectId: req.projectId,
      state: 'uploading',
      expiresAt: '2099-01-01T00:00:00Z',
      sources: req.sources.map((m, i) => ({ id: `${id}-${i}`, metadata: m, state: 'pending' })),
    })
    batches.set(id, batch)
    return create(CreateClipSourceBatchResponseSchema, {
      batch,
      uploads: batch.sources.map((s) => ({
        sourceId: s.id,
        putUrl: `https://storage.test/${s.id}`,
        headers: { 'Content-Type': s.metadata!.contentType, 'If-None-Match': '*' },
        expiresAt: '2099-01-01T00:00:00Z',
      })),
    })
  })
  router.rpc(ClipService.method.confirmClipSource, (req) => {
    options.calls?.push('ConfirmClipSource')
    options.sourceRequests?.push(req)
    if (options.confirmFails)
      throw connectAppError('CLIP_SOURCE_UNAVAILABLE', Code.FailedPrecondition)
    const b = batches.get(req.batchId)
    const source = b?.sources.find((s) => s.id === req.sourceId)
    if (!b || !source) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    source.state = 'ready'
    source.actualBytes = source.metadata!.bytes
    if (b.sources.every((s) => s.state === 'ready')) b.state = 'ready'
    return create(ConfirmClipSourceResponseSchema, { batch: b })
  })
  router.rpc(ClipService.method.discardClipSourceBatch, (req) => {
    options.calls?.push('DiscardClipSourceBatch')
    options.sourceRequests?.push(req)
    if (options.discardFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    batches.delete(req.batchId)
    return create(DiscardClipSourceBatchResponseSchema, {})
  })
}
