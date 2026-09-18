import { create } from '@bufbuild/protobuf'
import { Code, type ConnectError, type createRouterTransport } from '@connectrpc/connect'
import {
  ClipService,
  ClipRenderKind,
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
  GetClipCaptionPreviewResponseSchema,
  GetClipCaptionStyleSamplesResponseSchema,
  UpdateClipProjectResponseSchema,
  DeleteClipProjectResponseSchema,
  CreateClipSourceBatchResponseSchema,
  ConfirmClipSourceResponseSchema,
  DiscardClipSourceBatchResponseSchema,
  StartClipGenerationResponseSchema,
  QuoteClipGenerationResponseSchema,
  QuoteClipRevisionResponseSchema,
  StartClipRevisionResponseSchema,
  ReorderClipSourcesResponseSchema,
  SaveClipEditPlanResponseSchema,
  StartClipRenderResponseSchema,
  ClipEditingStateSchema,
  SeedPresetFieldsResponseSchema,
  type ProtoClipSourceBatch,
  type AppFailureReason,
} from '@/shared/api'
import type { ClipRecipe } from '@/entities/clip-template'
import { CLIP_CAPTION_STYLES, CLIP_DESIGN } from '@/shared/config'
import {
  clipPlanToProto,
  withSourceSound,
  compositionInputsToProto,
  toProjectComposition,
  toClipEditingState,
  type ClipProject,
  type ClipProjectDraft,
  type ClipEditPlan,
} from '@/entities/clip-project'
import { toFakeProto, type FakeGenerationJobRow } from './jobs'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]
export interface FakeClipTemplate extends ClipRecipe {
  /** The server converted this body from the old section grammar (CLIP-140). */
  compositionConverted?: boolean
  id: string
  projectCount?: number
  ownerId?: string
}
export interface FakeClipProject extends ClipProjectDraft {
  id: string
  composition?: ClipProject['composition']
  ownerId?: string
  finalized?: ClipProject['finalized']
  canFinalize?: boolean
  finalizationRefusal?: ClipProject['finalizationRefusal']
  result?: ClipProject['result']
  latestJob?: FakeGenerationJobRow
  latestAttempt?: ClipProject['latestAttempt']
  accounting?: ClipProject['accounting']
  editing?: ClipProject['editing']
  observations?: ClipProject['observations']
  attemptInspection?: ClipProject['attemptInspection']
  /** What the owner asked the AI for, newest first (CLIP-133). */
  requests?: ClipProject['requests']
  /** What the server states about the delivered plan (CLIP-109). */
  notices?: ClipProject['notices']
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
  finalize?: (
    input: { projectId: string; expectedRevision: number; expectedResultId: string },
    project: FakeClipProject,
  ) => Promise<void>
  cancel?: (
    input: { projectId: string; jobId: string },
    project: FakeClipProject,
  ) => Promise<FakeGenerationJobRow>
  compositionVersion?: number
  compositionPlanVersion?: number
  projects?: FakeClipProject[]
  readProject?: (project: FakeClipProject) => FakeClipProject
  generationStarts?: unknown[]
  generationJobId?: string
  generationFails?: boolean
  generationAmbiguous?: boolean
  generationReject?: AppFailureReason
  quoteRequests?: unknown[]
  quoteMaxCredits?: number
  sourceOrders?: string[][]
  quotePricedCalls?: { label: string; stage: string; calls: number }[]
  quoteExpiresAt?: string
  quoteFails?: AppFailureReason
  /** Every QuoteClipRevision/StartClipRevision request, in order. */
  revisionQuotes?: unknown[]
  revisionStarts?: unknown[]
  revisionJobId?: string
  revisionMaxCredits?: number
  revisionQuoteFails?: AppFailureReason
  revisionReject?: AppFailureReason
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
  soundWrites?: Array<{ sourceId: string; expectedRevision: number; retainOriginalAudio: boolean }>
  soundFails?: boolean
  planSaveConflict?: boolean
  projectWrites?: ClipProjectDraft[]
  /** Which styles the samples call back as sequence-rendered, and whether it fails at all. */
  sequenceStyles?: string[]
  /** The whole sequence-caption line a quote answers with, for a test that needs plan numbers. */
  sequenceCost?: {
    fromPlan: boolean
    captions: number
    frames: number
    addedRenderMs: number
    selectedStyles: number
  }
  captionSamplesFail?: boolean
  /** Holds UpdateClipProject open until the test releases it, so an assertion can run WHILE the
   *  settings autosave is in flight. */
  projectSaveGate?: () => Promise<unknown>
  projectSaveFails?: boolean
  projectSaveError?: ConnectError
  projectListFails?: boolean
  sourceRequests?: unknown[]
  retainedBatches?: ProtoClipSourceBatch[]
  sourceJobStatus?: (id: string) => string | undefined
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
/** A directory row states when it was last touched as a RELATIVE time (CLIP-41), and
 *  `formatRelativeTime` falls back to an absolute date once a week has passed. A literal date in a
 *  fixture therefore passes for a week and fails ever after, on a day nobody changed anything — so
 *  these stay a fixed distance from NOW instead of naming one.
 *
 *  Read ONCE per module load, never per response: a stamp that moved between two reads of the same
 *  project would make every refetch look like a change, and the settings form compares what the
 *  server last returned against its own baseline to decide whether it is in sync (CLIP-39). */
const LOADED_AT = Date.now()
const hoursAgo = (hours: number) => new Date(LOADED_AT - hours * 60 * 60 * 1000).toISOString()

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
      lastRenderKind: p.result
        ? p.result.renderKind === 'browser'
          ? ClipRenderKind.BROWSER
          : ClipRenderKind.SERVER
        : undefined,
      finalizedAt: p.finalized?.at,
      finalizedPlanRevision: p.finalized?.planRevision,
      finalizedResultId: p.finalized?.resultId,
      canEdit: !p.finalized,
      canFinalize:
        p.canFinalize ??
        (!!p.result?.id && !p.finalized && p.editPlanRevision === p.renderedPlanRevision),
      composition: p.composition
        ? {
            snapshot: p.composition.snapshot,
            inputs: compositionInputsToProto(p.composition.inputs),
          }
        : undefined,
      editing: p.editing
        ? create(ClipEditingStateSchema, { ...p.editing, plan: clipPlanToProto(p.editing.plan) })
        : undefined,
      result: p.result
        ? {
            ...p.result,
            renderKind:
              p.result.renderKind === 'browser' ? ClipRenderKind.BROWSER : ClipRenderKind.SERVER,
            bytes: BigInt(p.result.bytes),
          }
        : undefined,
      latestJob: p.latestJob ? toFakeProto(p.latestJob) : undefined,
      attemptInspection: p.attemptInspection,
      createdAt: hoursAgo(48),
      updatedAt: hoursAgo(2),
    })
  const rows = new Map(
    (options.templates ?? [])
      .filter((r) => !r.ownerId || r.ownerId === options.ownerId)
      .map((r) => [r.id, { ...r }]),
  )
  // What the server freezes for a project with no template: the grammar's own
  // minimum document (CLIP-5).
  const emptyCompositionBody = '<clip version="1"/>'
  const fixtureBody = (row: FakeClipTemplate) => {
    const escape = (v: string) =>
      v.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('"', '&quot;')
    return (
      `<clip version="1" pace="${row.captionPace ?? 'steady'}">` +
      row.informationFields
        .map(
          (f, i) =>
            `<field id="legacy_field_${i}" label="${escape(f.label)}" required="true">${escape(f.prompt)}</field>`,
        )
        .join('') +
      // The server reads a legacy body back converted (CLIP-140): its scene and
      // scene-bound caption are carried into the guide, never handed back as markup.
      `<guide>${escape(row.cutGuidance)}${escape(row.cutGuidance ? '\n\n' : '')}legacy-caption [ai]: Describe the selected scene.</guide><text id="intro" kind="fixed" role="hook"/><text id="outro" kind="fixed" role="ending"/></clip>`
    )
  }
  const toProto = (row: FakeClipTemplate) =>
    create(VideoTemplateSchema, {
      ...row,
      compositionBody: row.compositionBody ?? fixtureBody(row),
      compositionLegacy: row.compositionLegacy ?? !row.compositionBody,
      compositionConverted: row.compositionConverted ?? !row.compositionBody,
      projectCount: row.projectCount ?? 0,
      createdAt: hoursAgo(48),
      updatedAt: hoursAgo(2),
    })
  router.rpc(ClipService.method.listVideoTemplates, () => {
    options.calls?.push('ListVideoTemplates')
    if (options.listFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    return create(ListVideoTemplatesResponseSchema, { templates: [...rows.values()].map(toProto) })
  })
  router.rpc(ClipService.method.getClipCapabilities, () => {
    options.calls?.push('GetClipCapabilities')
    return {
      compositionVersion: options.compositionVersion ?? 1,
      compositionPlanVersion: options.compositionPlanVersion ?? 5,
    }
  })
  let next = 0
  router.rpc(ClipService.method.createVideoTemplate, (req) => {
    options.calls?.push('CreateVideoTemplate')
    if (options.saveFails) throw connectAppError('CLIP_TEMPLATE_NAME_TAKEN', Code.AlreadyExists)
    const row: FakeClipTemplate = {
      id: `video-template-${++next}`,
      name: req.name.trim(),
      compositionBody: req.compositionBody || undefined,
      compositionLegacy: false,
      cutGuidance: req.cutGuidance,
      informationFields: req.informationFields.map(({ label, prompt }) => ({ label, prompt })),

      accent: req.accent as ClipRecipe['accent'],
      preset: req.preset as ClipRecipe['preset'],
      captionPace: (req.captionPace || 'steady') as ClipRecipe['captionPace'],
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
    if (req.compositionBody !== undefined) {
      row.compositionBody = req.compositionBody
      row.compositionLegacy = false
    }
    if (req.cutGuidance !== undefined) row.cutGuidance = req.cutGuidance
    if (req.informationFields)
      row.informationFields = req.informationFields.values.map(({ label, prompt }) => ({
        label,
        prompt,
      }))

    if (req.accent !== undefined) row.accent = req.accent as ClipRecipe['accent']
    if (req.preset !== undefined) row.preset = req.preset as ClipRecipe['preset']
    if (req.captionPace !== undefined)
      row.captionPace = (req.captionPace || 'steady') as ClipRecipe['captionPace']
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
      projects: [...projects.values()].map((p) => projectProto(options.readProject?.(p) ?? p)),
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
  router.rpc(ClipService.method.finalizeClipProject, async (req) => {
    options.calls?.push('FinalizeClipProject')
    const p = projects.get(req.projectId)
    if (!p) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    if (options.finalize) await options.finalize(req, p)
    else {
      if (
        req.expectedRevision !== p.editPlanRevision ||
        req.expectedRevision !== p.renderedPlanRevision ||
        req.expectedResultId !== p.result?.id
      )
        throw connectAppError('CLIP_FINALIZATION_CONFLICT', Code.Aborted)
      p.finalized = {
        at: '2026-09-13T00:00:00Z',
        planRevision: req.expectedRevision,
        resultId: req.expectedResultId,
      }
      p.editing = undefined
      p.observations = undefined
    }
    return { project: projectProto(p) }
  })
  router.rpc(ClipService.method.cancelClipJob, async (req) => {
    options.calls?.push('CancelClipJob')
    const p = projects.get(req.projectId)
    if (!p) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    if (!options.cancel) throw connectAppError('CLIP_BUSY', Code.FailedPrecondition)
    const job = await options.cancel(req, p)
    return { job: toFakeProto(job), accepted: !!job.cancelRequestedAt }
  })
  /** The captions of the plan being edited, drawn as the renderer draws them (CDS-83). The fake
   *  draws a box per caption at a fixed spot: what a test can check here is that ② places the
   *  fragment it was given and asks for it once, not how the glyphs look. */
  router.rpc(ClipService.method.getClipCaptionPreview, (req) => {
    options.calls?.push('GetClipCaptionPreview')
    const p = projects.get(req.projectId)
    if (!p) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    return create(GetClipCaptionPreviewResponseSchema, {
      ratio: p.ratio,
      canvas: { x: 0, y: 0, width: 1080, height: 1920 },
      safeArea: { x: 60, y: 120, width: 960, height: 1680 },
      captions: (req.plan?.elements ?? [])
        .filter((text) => text.role === 'caption')
        .map((text) => ({
          instanceId: text.instanceId,
          style: text.ownerStyle || text.style,
          svg: `<g data-caption="${text.instanceId}"><text>${text.text}</text></g>`,
          box: { x: 240, y: 1500, width: 600, height: 120 },
          fontSize: 72,
          representativeFrame: (options.sequenceStyles ?? ['word-pop']).includes(
            text.ownerStyle || text.style,
          ),
        })),
    })
  })
  /** Every approved style drawn once. The fake draws a box, not the style: what a test can check
   *  here is that ① offers each style, says which ones are drawn frame by frame, and saves what
   *  was ticked — the drawing itself is the renderer's, pinned by its own Go contract. */
  router.rpc(ClipService.method.getClipCaptionStyleSamples, (req) => {
    options.calls?.push('GetClipCaptionStyleSamples')
    const p = projects.get(req.projectId)
    if (!p) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    if (options.captionSamplesFail) throw connectAppError('CLIP_BUSY', Code.FailedPrecondition)
    return create(GetClipCaptionStyleSamplesResponseSchema, {
      ratio: p.ratio,
      canvas: { x: 0, y: 0, width: 1080, height: 1920 },
      samples: CLIP_CAPTION_STYLES.map((style) => ({
        instanceId: style,
        style,
        svg: `<g data-style="${style}"><text>오늘의 한 장면</text></g>`,
        box: { x: 0, y: 0, width: 600, height: 120 },
        fontSize: 72,
        representativeFrame: (options.sequenceStyles ?? ['word-pop']).includes(style),
      })),
    })
  })
  router.rpc(ClipService.method.createClipProject, (req) => {
    options.calls?.push('CreateClipProject')
    if (options.projectSaveFails) throw connectAppError('CLIP_INVALID_INPUT', Code.InvalidArgument)
    const p: FakeClipProject = {
      id: `clip-${++next}`,
      title: req.title,
      videoTemplateId: req.videoTemplateId,
      ratio: req.ratio as ClipProjectDraft['ratio'],
      targetDurationMs: req.targetDurationMs,
      answers: req.answers.map((a) => ({ label: a.label, text: a.text })),
      disclosure: req.disclosure as ClipProjectDraft['disclosure'],
      hideDisclosure: req.hideDisclosure,
      cta: req.cta as ClipProjectDraft['cta'],
      instruction: req.instruction,
      // A template seeds none of the design: every project starts unset, which
      // is the shared default (CLIP-14, CLIP-139).
      ...(req.introPreset !== undefined
        ? { introPreset: req.introPreset as ClipProjectDraft['introPreset'] }
        : {}),
      ...(req.outroPreset !== undefined
        ? { outroPreset: req.outroPreset as ClipProjectDraft['outroPreset'] }
        : {}),
      ...(req.allowedCaptionStyles
        ? { allowedCaptionStyles: [...req.allowedCaptionStyles.values] }
        : {}),
    }
    if (req.compositionInputs) {
      const template = rows.get(p.videoTemplateId)!
      const wire = create(ClipProjectSchema, {
        composition: {
          snapshot: {
            version: 1,
            body: template.compositionBody ?? fixtureBody(template),
            templateId: template.id,
            legacy: template.compositionLegacy ?? !template.compositionBody,
          },
          inputs: req.compositionInputs,
        },
      })
      p.composition = toProjectComposition(wire.composition)
      p.compositionInputs = p.composition?.inputs
    }
    projects.set(p.id, p)
    options.projectWrites?.push(p)
    return create(CreateClipProjectResponseSchema, { project: projectProto(p) })
  })
  router.rpc(ClipService.method.updateClipProject, async (req) => {
    options.calls?.push('UpdateClipProject')
    if (options.projectSaveGate) await options.projectSaveGate()
    if (options.projectSaveError) throw options.projectSaveError
    if (options.projectSaveFails) throw connectAppError('CLIP_INVALID_INPUT', Code.InvalidArgument)
    const p = projects.get(req.id)
    if (!p) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    if (req.title !== undefined) p.title = req.title
    if (req.videoTemplateId !== undefined) p.videoTemplateId = req.videoTemplateId
    if (req.targetDurationMs !== undefined) p.targetDurationMs = req.targetDurationMs
    if (req.hideDisclosure !== undefined) p.hideDisclosure = req.hideDisclosure
    if (req.disclosure !== undefined)
      p.disclosure = req.disclosure as ClipProjectDraft['disclosure']
    if (req.cta !== undefined) p.cta = req.cta as ClipProjectDraft['cta']
    if (req.instruction !== undefined) p.instruction = req.instruction
    if (req.captionPace !== undefined)
      p.captionPace = req.captionPace as ClipProjectDraft['captionPace']
    if (req.accent !== undefined) p.accent = req.accent as ClipProjectDraft['accent']
    if (req.introPreset !== undefined)
      p.introPreset = req.introPreset as ClipProjectDraft['introPreset']
    if (req.outroPreset !== undefined)
      p.outroPreset = req.outroPreset as ClipProjectDraft['outroPreset']
    if (req.allowedCaptionStyles) p.allowedCaptionStyles = [...req.allowedCaptionStyles.values]
    for (const answer of req.answers)
      p.answers = [
        ...p.answers.filter((a) => a.label !== answer.label),
        { label: answer.label, text: answer.text },
      ]
    if (req.compositionInputs) {
      // A project with no template freezes the grammar's minimum document, and
      // there is no row to read a body from (CLIP-5, T218).
      const template = p.videoTemplateId ? rows.get(p.videoTemplateId)! : undefined
      const snapshot = !template
        ? (p.composition?.snapshot ?? {
            version: 1,
            body: emptyCompositionBody,
            legacy: false,
          })
        : p.composition?.snapshot.templateId === p.videoTemplateId &&
            (!template.compositionBody || template.compositionLegacy)
          ? p.composition.snapshot
          : {
              version: 1,
              body: template.compositionBody ?? fixtureBody(template),
              templateId: template.id,
              legacy: template.compositionLegacy ?? !template.compositionBody,
            }
      const wire = create(ClipProjectSchema, {
        composition: { snapshot, inputs: req.compositionInputs },
      })
      p.composition = toProjectComposition(wire.composition)
      p.compositionInputs = p.composition?.inputs
    }
    options.projectWrites?.push({ ...p })
    return create(UpdateClipProjectResponseSchema, { project: projectProto(p) })
  })
  router.rpc(ClipService.method.deleteClipProject, (req) => {
    options.calls?.push('DeleteClipProject')
    if (options.deleteFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    if (!projects.delete(req.id)) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    return create(DeleteClipProjectResponseSchema, {})
  })
  const batches = new Map<string, ProtoClipSourceBatch>(
    (options.retainedBatches ?? []).map((b) => [b.id, b]),
  )
  const quotes = new Map<string, { id: string; max: number }>()
  const revisionQuotes = new Map<
    string,
    { id: string; max: number; request: string; target: string; revision: number }
  >()
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
  /** What the server states about frame-by-frame captions on an approval surface
   *  (CDS-81). The fake counts only the SELECTION, which is what a project with
   *  no plan yet knows; a test that needs plan numbers supplies them itself. */
  const sequenceCost = (p?: FakeClipProject) => ({
    ...(options.sequenceCost ?? {
      fromPlan: false,
      captions: 0,
      frames: 0,
      addedRenderMs: 0,
      selectedStyles: (p?.allowedCaptionStyles ?? []).filter((style) =>
        (options.sequenceStyles ?? ['word-pop']).includes(style),
      ).length,
    }),
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
      cancellationPolicy: {
        version: 1,
        unusedReservationNumerator: 1,
        unusedReservationDenominator: 2,
        rounding: 'ceil',
      },
      expiresAt: options.quoteExpiresAt ?? '2099-01-01T00:00:00Z',
      // The observation and the two writing calls a generation makes.
      pricedCalls: options.quotePricedCalls ?? [
        { label: 'observe', stage: 'observe', calls: 3 },
        { label: 'flow', stage: 'write', calls: 1 },
        { label: 'narration', stage: 'write', calls: 1 },
      ],
      sequenceCaptions: sequenceCost(projects.get(req.projectId)),
    })
  })
  // A revision is quoted and started like a generation, but against the SAVED
  // PLAN rather than a batch: the quote binds the plan revision it was taken
  // against, and the narration target pays for one writing call instead of two.
  router.rpc(ClipService.method.quoteClipRevision, (req) => {
    options.calls?.push('QuoteClipRevision')
    options.revisionQuotes?.push(req)
    if (options.revisionQuoteFails)
      throw connectAppError(options.revisionQuoteFails, Code.FailedPrecondition)
    const p = projects.get(req.projectId)
    if (!p?.editing) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    if (p.latestJob && !['done', 'failed', 'cancelled'].includes(p.latestJob.status ?? ''))
      throw connectAppError('CLIP_BUSY', Code.FailedPrecondition)
    if (!req.request.trim() || !['flow', 'narration', 'both'].includes(req.target))
      throw connectAppError('CLIP_INVALID_INPUT', Code.InvalidArgument)
    const q = {
      id: `revision-quote-${++quoteNumber}`,
      max: options.revisionMaxCredits ?? (req.target === 'narration' ? 8 : 16),
      request: req.request,
      target: req.target,
      revision: p.editPlanRevision ?? 0,
    }
    revisionQuotes.set(q.id, q)
    return create(QuoteClipRevisionResponseSchema, {
      quoteId: q.id,
      maxCredits: q.max,
      planRevision: q.revision,
      cancellationPolicy: {
        version: 1,
        unusedReservationNumerator: 1,
        unusedReservationDenominator: 2,
        rounding: 'ceil',
      },
      expiresAt: options.quoteExpiresAt ?? '2099-01-01T00:00:00Z',
      pricedCalls:
        req.target === 'narration'
          ? [{ label: 'narration', stage: 'write', calls: 1 }]
          : [
              { label: 'flow', stage: 'write', calls: 1 },
              { label: 'narration', stage: 'write', calls: 1 },
            ],
      sequenceCaptions: sequenceCost(projects.get(req.projectId)),
    })
  })
  router.rpc(ClipService.method.startClipRevision, (req) => {
    options.calls?.push('StartClipRevision')
    options.revisionStarts?.push(req)
    if (options.revisionReject)
      throw connectAppError(options.revisionReject, Code.FailedPrecondition)
    const p = projects.get(req.projectId)
    if (!p?.editing) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    const quote = revisionQuotes.get(req.quoteId)
    if (!quote || quote.max !== req.approvedMaxCredits)
      throw connectAppError('CLIP_QUOTE_REQUIRED', Code.FailedPrecondition)
    if (quote.request !== req.request || quote.target !== req.target)
      throw connectAppError('CLIP_QUOTE_CHANGED', Code.FailedPrecondition)
    if (quote.revision !== (p.editPlanRevision ?? 0))
      throw connectAppError('CLIP_QUOTE_CHANGED', Code.FailedPrecondition)
    const jobId = options.revisionJobId ?? 'clip-revision-job'
    p.latestJob = {
      id: jobId,
      kind: 'revise_clip',
      status: 'queued',
      stage: 'flow',
      clipProjectId: p.id,
    }
    p.latestAttempt = { jobId, batchId: '', quoteId: quote.id }
    p.accounting = {
      jobId,
      status: 'not_reserved',
      approvedMaxCredits: quote.max,
      reservedCredits: 0,
      settled: false,
    }
    return create(StartClipRevisionResponseSchema, { jobId })
  })
  router.rpc(ClipService.method.reorderClipSources, (req) => {
    options.calls?.push('ReorderClipSources')
    options.sourceOrders?.push([...req.sourceIds])
    const batch = batches.get(req.batchId)
    if (!batch || batch.projectId !== req.projectId)
      throw connectAppError('CLIP_SOURCE_UNAVAILABLE', Code.FailedPrecondition)
    const held = batch.sources.map((s) => s.id)
    if (
      req.sourceIds.length !== held.length ||
      new Set(req.sourceIds).size !== req.sourceIds.length ||
      req.sourceIds.some((id) => !held.includes(id))
    )
      throw connectAppError('CLIP_INVALID_INPUT', Code.InvalidArgument)
    batch.sources = req.sourceIds.map((id) => batch.sources.find((s) => s.id === id)!)
    return create(ReorderClipSourcesResponseSchema, { batch })
  })
  router.rpc(ClipService.method.setClipSourceOriginalSound, (req) => {
    options.calls?.push('SetClipSourceOriginalSound')
    options.soundWrites?.push(req)
    const p = projects.get(req.projectId),
      b = batches.get(req.batchId)
    const source = b?.sources.find(
      (s) => s.id === req.sourceId && s.metadata?.fingerprint === req.expectedFingerprint,
    )
    if (!p || !b?.current || !source) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    if (options.soundFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    if (req.expectedRevision !== (p.editPlanRevision ?? 0))
      throw connectAppError('CLIP_PLAN_CONFLICT', Code.Aborted)
    if (source.retainOriginalAudio !== req.retainOriginalAudio) {
      source.retainOriginalAudio = req.retainOriginalAudio
      if (p.editing) {
        if (p.editing.plan.cuts.some((c) => c.sourceId === source.id))
          p.editing.plan = withSourceSound(p.editing.plan, {
            sourceId: source.id,
            fingerprint: req.expectedFingerprint,
            retainOriginalAudio: req.retainOriginalAudio,
          })
        p.editPlanRevision = (p.editPlanRevision ?? 0) + 1
      }
    }
    return { project: projectProto(p), batch: b }
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
  router.rpc(ClipService.method.getClipSources, (req) => {
    options.calls?.push('GetClipSources')
    if (!projects.has(req.projectId)) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    const p = projects.get(req.projectId)!
    const current = options.readProject?.(p) ?? p
    const status =
      options.sourceJobStatus?.(current.latestJob?.id ?? '') ?? current.latestJob?.status
    for (const b of batches.values())
      if (
        b.projectId === req.projectId &&
        b.state === 'consuming' &&
        (status === 'done' || status === 'failed' || status === 'cancelled')
      )
        b.state = 'ready'
    return { batches: [...batches.values()].filter((b) => b.projectId === req.projectId) }
  })
  router.rpc(ClipService.method.getClipSourcePlayback, (req) => {
    const b = [...batches.values()].find((b) => b.current && b.projectId === req.projectId)
    const source = b?.sources.find(
      (s) => s.id === req.sourceId && s.metadata?.fingerprint === req.expectedFingerprint,
    )
    if (!source) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    return {
      url: `https://storage.test/play/${source.id}`,
      expiresAt: new Date(Date.now() + 300000).toISOString(),
    }
  })
  router.rpc(ClipService.method.createClipSourceBatch, (req) => {
    options.calls?.push('CreateClipSourceBatch')
    options.sourceRequests?.push(req)
    if (options.reserveFails)
      throw connectAppError('CLIP_SOURCE_UNAVAILABLE', Code.FailedPrecondition)
    if (!projects.has(req.projectId)) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    for (const b of batches.values()) if (b.projectId === req.projectId) b.current = false
    const id = `batch-${++next}`
    const batch = create(ClipSourceBatchSchema, {
      id,
      projectId: req.projectId,
      state: 'uploading',
      current: true,
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
    source.availability = 'available'
    source.retentionExpiresAt = b.expiresAt
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
