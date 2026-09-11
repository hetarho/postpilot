import { create } from '@bufbuild/protobuf'
import { Code, type createRouterTransport } from '@connectrpc/connect'
import {
  ClipService,
  ClipAnalysisEligibility,
  ListClipAnalysisEligibilityResponseSchema,
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
  QuoteClipGenerationResponseSchema,
  SaveClipEditPlanResponseSchema,
  StartClipRenderResponseSchema,
  ClipEditingStateSchema,
  SeedPresetFieldsResponseSchema,
  type ProtoClipSourceBatch,
  type AppFailureReason,
} from '@/shared/api'
import type { ClipRecipe } from '@/entities/clip-template'
import { CLIP_DESIGN } from '@/shared/config'
import {
  clipPlanToProto,
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
  latestAttempt?: ClipProject['latestAttempt']
  accounting?: ClipProject['accounting']
  editing?: ClipProject['editing']
  editPlanRevision?: number
  renderedPlanRevision?: number
}
export type FakeClipEligibility =
  | 'unspecified'
  | 'eligible'
  | 'video_input_absent'
  | 'inline_endpoint_unavailable'
  | 'required_parameters_unsupported'
  | 'price_ceiling_unavailable'

const ELIGIBILITY_WIRE: Record<FakeClipEligibility, ClipAnalysisEligibility> = {
  unspecified: ClipAnalysisEligibility.UNSPECIFIED,
  eligible: ClipAnalysisEligibility.ELIGIBLE,
  video_input_absent: ClipAnalysisEligibility.VIDEO_INPUT_ABSENT,
  inline_endpoint_unavailable: ClipAnalysisEligibility.INLINE_ENDPOINT_UNAVAILABLE,
  required_parameters_unsupported: ClipAnalysisEligibility.REQUIRED_PARAMETERS_UNSUPPORTED,
  price_ceiling_unavailable: ClipAnalysisEligibility.PRICE_CEILING_UNAVAILABLE,
}

export interface FakeClipsOptions {
  projects?: FakeClipProject[]
  readProject?: (project: FakeClipProject) => FakeClipProject
  generationStarts?: unknown[]
  generationJobId?: string
  generationFails?: boolean
  generationAmbiguous?: boolean
  generationReject?: AppFailureReason
  quoteRequests?: unknown[]
  quoteMaxCredits?: number
  quoteExpiresAt?: string
  quoteFails?: AppFailureReason
  /** T111's live eligibility answer, one row per registered observe model. Absent means the
   *  server names nobody, which the page must read as "unresolved", never as eligible. */
  eligibility?: Array<{ providerId: string; modelId: string; status: FakeClipEligibility }>
  /** Make ListClipAnalysisEligibility fail; a function is read on every call, so a test can
   *  let a retry succeed. */
  eligibilityFails?: boolean | (() => boolean)
  renderStarts?: unknown[]
  renderJobId?: string
  renderFails?: boolean
  /** A verifier refusal, as the server's stable reason (CDS-52, LANG-21). */
  renderRefusal?: AppFailureReason
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
  // `editing` is the domain shape, whose caption carries an anchor; the wire
  // still calls that field `position` (CDS-12), so it goes through the mapper.
  const projectProto = (p: FakeClipProject) =>
    create(ClipProjectSchema, {
      ...p,
      editing: p.editing
        ? create(ClipEditingStateSchema, { ...p.editing, plan: clipPlanToProto(p.editing.plan) })
        : undefined,
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
      preset: req.preset as ClipRecipe['preset'],
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
    if (req.preset !== undefined) row.preset = req.preset as ClipRecipe['preset']
    options.writes?.push({ ...row })
    return create(UpdateVideoTemplateResponseSchema, { template: toProto(row) })
  })
  router.rpc(ClipService.method.seedPresetFields, (req) => {
    options.calls?.push('SeedPresetFields')
    const preset = CLIP_DESIGN.presets[req.preset as keyof typeof CLIP_DESIGN.presets]
    if (!preset) throw connectAppError('CLIP_INVALID_INPUT', Code.InvalidArgument)
    const labels = ['상호', ...preset.chips].filter((l, i, all) => all.indexOf(l) === i)
    return create(SeedPresetFieldsResponseSchema, {
      fields: labels.map((label) => ({
        label,
        prompt: CLIP_DESIGN.facts.prompts[label as keyof typeof CLIP_DESIGN.facts.prompts],
      })),
    })
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
      disclosure: req.disclosure as ClipProjectDraft['disclosure'],
      cta: req.cta as ClipProjectDraft['cta'],
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
    if (req.disclosure !== undefined)
      p.disclosure = req.disclosure as ClipProjectDraft['disclosure']
    if (req.cta !== undefined) p.cta = req.cta as ClipProjectDraft['cta']
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
  const quotes = new Map<string, { id: string; max: number }>()
  let quoteNumber = 0
  router.rpc(ClipService.method.listClipAnalysisEligibility, () => {
    options.calls?.push('ListClipAnalysisEligibility')
    const fails =
      typeof options.eligibilityFails === 'function'
        ? options.eligibilityFails()
        : options.eligibilityFails
    if (fails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    return create(ListClipAnalysisEligibilityResponseSchema, {
      models: (options.eligibility ?? []).map((row) => ({
        model: { providerId: row.providerId, modelId: row.modelId },
        status: ELIGIBILITY_WIRE[row.status],
      })),
    })
  })
  router.rpc(ClipService.method.quoteClipGeneration, (req) => {
    options.calls?.push('QuoteClipGeneration')
    options.quoteRequests?.push(req)
    if (options.quoteFails) throw connectAppError(options.quoteFails, Code.FailedPrecondition)
    const batch = batches.get(req.batchId)
    if (!batch || batch.projectId !== req.projectId || batch.state !== 'ready')
      throw connectAppError('CLIP_SOURCE_UNAVAILABLE', Code.FailedPrecondition)
    const q = { id: `quote-${++quoteNumber}`, max: options.quoteMaxCredits ?? 20 }
    quotes.set(batch.id, q)
    return create(QuoteClipGenerationResponseSchema, {
      quoteId: q.id,
      maxCredits: q.max,
      expiresAt: options.quoteExpiresAt ?? '2099-01-01T00:00:00Z',
    })
  })
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
    if (options.renderRefusal) throw connectAppError(options.renderRefusal, Code.InvalidArgument)
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
    p.latestAttempt = { jobId, batchId: b.id, quoteId: '' }
    return create(StartClipRenderResponseSchema, { jobId })
  })
  router.rpc(ClipService.method.startClipGeneration, (req) => {
    options.calls?.push('StartClipGeneration')
    options.generationStarts?.push(req)
    if (options.generationFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    if (options.generationReject)
      throw connectAppError(options.generationReject, Code.FailedPrecondition)
    const p = projects.get(req.projectId)
    const b = batches.get(req.batchId)
    if (!p || !b || b.state !== 'ready')
      throw connectAppError('CLIP_SOURCE_UNAVAILABLE', Code.FailedPrecondition)
    const quote = quotes.get(b.id)
    if (!quote || quote.id !== req.quoteId || quote.max !== req.approvedMaxCredits)
      throw connectAppError('CLIP_QUOTE_REQUIRED', Code.FailedPrecondition)
    b.state = 'consuming'
    const jobId = options.generationJobId ?? 'clip-job'
    p.latestJob = {
      id: jobId,
      kind: 'generate_clip',
      status: 'queued',
      stage: 'prepare',
      clipProjectId: p.id,
    }
    p.latestAttempt = { jobId, batchId: b.id, quoteId: quote.id }
    p.accounting = {
      jobId,
      status: 'not_reserved',
      approvedMaxCredits: quote.max,
      reservedCredits: 0,
      settled: false,
    }
    if (options.generationAmbiguous) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
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
