import { create } from '@bufbuild/protobuf'
import { Code, type ConnectError, type createRouterTransport } from '@connectrpc/connect'
import {
  ClipGenerationService,
  ClipPlanService,
  ClipRenderService,
  ClipSourceService,
  ClipTemplateService,
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
  GetClipRegionPresetSamplesResponseSchema,
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
  type ProtoClipSourceBatch,
  type AppFailureReason,
} from '@/shared/api'
import type { ClipRecipe, ClipTemplateDesign } from '@/entities/clip-template'
import {
  CLIP_CAPTION_STYLES,
  CLIP_DEFAULT_REGION_PRESETS,
  CLIP_DESIGN,
  clipRegionSlots,
} from '@/entities/clip-design'
import {
  clipPlanToProto,
  toClipEditingState,
  type ClipEditPlan,
  type ClipEditableText,
  withSourceSound,
} from '@/entities/clip-plan'
import {
  CLIP_PROJECT_LIMITS,
  compositionInputsToProto,
  toProjectComposition,
  type ClipProject,
  type ClipProjectDraft,
} from '@/entities/clip-project'
import { toFakeProto, type FakeGenerationJobRow } from './jobs'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]
/** A template row; a fixture naming no design stands at the shared defaults, as a migrated
 *  row does (CLIP-166). */
export interface FakeClipTemplate
  extends Omit<ClipRecipe, keyof ClipTemplateDesign>, Partial<ClipTemplateDesign> {
  id: string
  projectCount?: number
  ownerId?: string
}
function designed(row: FakeClipTemplate): FakeClipTemplate & ClipTemplateDesign {
  return {
    ...row,
    introPreset: row.introPreset ?? CLIP_DEFAULT_REGION_PRESETS.intro,
    outroPreset: row.outroPreset ?? CLIP_DEFAULT_REGION_PRESETS.outro,
    allowedCaptionStyles: [...(row.allowedCaptionStyles ?? [])],
  }
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
  /** The clip's storyline (CLIP-178), in the wire's shape. */
  storyline?: {
    paragraphs: Array<{ text: string; observationIds: string[] }>
    editedByHand: boolean
    addedSourceIds: string[]
    takenOutObservationIds: string[]
  }
  planEditedByHand?: boolean
  /** The project's intro and outro slots (CLIP-186), in the domain's shape. */
  regions?: ClipProject['regions']
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
  /** Every StartClipStoryline as it arrived, and the job id it answers with (CLIP-177). */
  storylineStarts?: unknown[]
  storylineJobId?: string
  /** Every storyline edit an UpdateClipProject carried (CLIP-178). */
  storylineEdits?: Array<Array<{ text: string; observationIds: string[] }>>
  /** Refuse every storyline edit as not keeping its shape. */
  storylineEditFails?: boolean
  /** Every QuoteClipStorylineRevision and StartClipStorylineRevision as it arrived (CLIP-181). */
  storylineRequestQuotes?: unknown[]
  storylineRequestStarts?: unknown[]
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
  /** Holds SaveClipEditPlan open until the test releases it. */
  planSaveGate?: () => Promise<unknown>
  /** Every region edit an UpdateClipProject carried, with the revision it named (CLIP-188). */
  regionWrites?: Array<{
    expectedRegionRevision?: number
    intro?: { enabled?: boolean; slots: { id: string; instruction?: string; text?: string }[] }
    outro?: { enabled?: boolean; slots: { id: string; instruction?: string; text?: string }[] }
  }>
  /** Holds an UpdateClipProject that carries region edits open until the test releases it. */
  regionSaveGate?: () => Promise<unknown>
  /** What choosing a template seeds into the slots the owner has not fixed, by template id
   *  (CLIP-168); a region it seeds is turned on. */
  templateRegions?: Record<string, { intro?: string[]; outro?: string[] }>
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
  /** GetClipRegionPresetSamples fails, so ① shows the presets by name alone. */
  regionSamplesFail?: boolean
  /** Every slot label ① asked the region samples for. */
  regionSampleLabels?: string[]
  /** Holds UpdateClipProject open until the test releases it, so an assertion can run WHILE the
   *  settings autosave is in flight. */
  projectSaveGate?: () => Promise<unknown>
  projectSaveFails?: boolean
  projectSaveError?: ConnectError
  projectListFails?: boolean
  sourceRequests?: unknown[]
  retainedBatches?: ProtoClipSourceBatch[]
  /** What a generation quote reports it can reuse from the previous attempt (CLIP-96). */
  quoteRecovery?: {
    reusedChunks?: number
    remainingChunks?: number
    renderOnly?: boolean
    responseRetries?: number
  }
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

const REGION_KINDS = ['intro', 'outro'] as const
const regionRole = (kind: 'intro' | 'outro') => (kind === 'intro' ? 'hook' : 'ending')
const presetOf = (p: FakeClipProject, kind: 'intro' | 'outro') =>
  (kind === 'intro' ? p.introPreset : p.outroPreset) || CLIP_DEFAULT_REGION_PRESETS[kind]

/** The element the server projects a region's slots into (T451): one row per active slot. */
function regionElement(kind: 'intro' | 'outro'): ClipEditableText {
  return {
    instanceId: `project-${kind}`,
    elementId: `project-${kind}`,
    cutId: '',
    kind: 'fixed',
    role: regionRole(kind),
    text: '',
    rows: [],
    style: 'auto',
    position: 'auto',
    align: 'center',
    basis: kind === 'intro' ? 'output-start' : 'output-end',
    startMs: kind === 'intro' ? 0 : -3000,
    endMs: kind === 'intro' ? 2500 : 0,
    pace: '',
    accent: '',
    keyword: '',
    resolvedStartMs: 0,
    resolvedEndMs: 0,
    groupId: '',
    itemId: '',
  }
}

/** The server's projection of the slots into the plan (CLIP-188): an enabled region with text
 *  draws its active slots as one element's rows, and anything else draws none. The plan's
 *  revision moves only when the drawing does. */
export function projectFakeRegions(p: FakeClipProject): boolean {
  if (!p.regions || !p.editing) return false
  const elements = [...(p.editing.plan.elements ?? [])]
  const before = JSON.stringify(elements)
  for (const kind of REGION_KINDS) {
    const id = `project-${kind}`
    const specs = clipRegionSlots(kind, presetOf(p, kind))
    const region = p.regions[kind]
    const rows = region.slots.slice(0, specs.length).map((slot, i) => ({
      role: specs[i].role,
      text: slot.text,
    }))
    const at = elements.findIndex((t) => t.instanceId === id)
    if (!region.enabled || !rows.some((row) => row.text.trim())) {
      if (at >= 0) elements.splice(at, 1)
      continue
    }
    const element = { ...(at >= 0 ? elements[at] : regionElement(kind)), rows }
    if (at >= 0) elements[at] = element
    else if (kind === 'intro') elements.unshift(element)
    else elements.push(element)
  }
  if (JSON.stringify(elements) === before) return false
  p.editing = { ...p.editing, plan: { ...p.editing.plan, elements } }
  p.editPlanRevision = (p.editPlanRevision ?? 0) + 1
  return true
}

/** A correction's region rows written back into their slots (T451): a changed row is the
 *  owner's, and a removed element turns its region off. */
function reconcileFakeRegions(p: FakeClipProject, before: ClipEditPlan, after: ClipEditPlan) {
  const regions = p.regions
  if (!regions) return
  const next = { ...regions }
  for (const kind of REGION_KINDS) {
    const id = `project-${kind}`
    const old = before.elements?.find((t) => t.instanceId === id)
    const saved = after.elements?.find((t) => t.instanceId === id)
    const region = regions[kind]
    if (old && !saved) next[kind] = { ...region, enabled: false }
    if (!saved) continue
    next[kind] = {
      ...region,
      slots: region.slots.map((slot, i) => {
        const row = saved.rows[i]
        if (!row || row.text === (old?.rows[i]?.text ?? '')) return slot
        return { ...slot, text: row.text, ownerFixed: true, bound: false }
      }),
    }
  }
  if (JSON.stringify(next) !== JSON.stringify(regions))
    p.regions = { ...next, revision: regions.revision + 1 }
}

export function registerClipService(router: ConnectRouter, options: FakeClipsOptions = {}) {
  const projects = new Map<string, FakeClipProject>(
    (options.projects ?? [])
      .filter((p) => !p.ownerId || p.ownerId === options.ownerId)
      .map((p) => [p.id, { ...p }]),
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
      .map((r) => [r.id, designed(r)]),
  )
  // What the server freezes for a project with no template: the grammar's own
  // minimum document (CLIP-5).
  const emptyCompositionBody = '<clip version="1"/>'
  const templateFor = (id: string) => (id ? rows.get(id) : undefined)
  const toProto = (row: FakeClipTemplate) =>
    create(VideoTemplateSchema, {
      ...row,
      projectCount: row.projectCount ?? 0,
      createdAt: hoursAgo(48),
      updatedAt: hoursAgo(2),
    })
  router.rpc(ClipTemplateService.method.listVideoTemplates, () => {
    options.calls?.push('ListVideoTemplates')
    if (options.listFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    return create(ListVideoTemplatesResponseSchema, { templates: [...rows.values()].map(toProto) })
  })
  router.rpc(ClipTemplateService.method.getClipCapabilities, () => {
    options.calls?.push('GetClipCapabilities')
    return {
      compositionVersion: options.compositionVersion ?? 1,
      compositionPlanVersion: options.compositionPlanVersion ?? 5,
    }
  })
  let next = 0
  router.rpc(ClipTemplateService.method.createVideoTemplate, (req) => {
    options.calls?.push('CreateVideoTemplate')
    if (options.saveFails) throw connectAppError('CLIP_TEMPLATE_NAME_TAKEN', Code.AlreadyExists)
    // A template is an outline body under a name; a request without one is refused (CLIP-14).
    if (!req.compositionBody.trim())
      throw connectAppError('CLIP_INVALID_INPUT', Code.InvalidArgument)
    const row = designed({
      id: `video-template-${++next}`,
      name: req.name.trim(),
      compositionBody: req.compositionBody,
      ...(req.introPreset
        ? { introPreset: req.introPreset as ClipTemplateDesign['introPreset'] }
        : {}),
      ...(req.outroPreset
        ? { outroPreset: req.outroPreset as ClipTemplateDesign['outroPreset'] }
        : {}),
      ...(req.allowedCaptionStyles
        ? { allowedCaptionStyles: [...req.allowedCaptionStyles.values] }
        : {}),
    })
    options.writes?.push(row)
    rows.set(row.id, row)
    return create(CreateVideoTemplateResponseSchema, { template: toProto(row) })
  })
  router.rpc(ClipTemplateService.method.updateVideoTemplate, (req) => {
    options.calls?.push('UpdateVideoTemplate')
    if (options.saveFails) throw connectAppError('CLIP_TEMPLATE_NAME_TAKEN', Code.AlreadyExists)
    const row = rows.get(req.id)
    if (!row) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    if (req.name !== undefined) row.name = req.name
    if (req.compositionBody !== undefined) row.compositionBody = req.compositionBody
    if (req.introPreset !== undefined)
      row.introPreset = req.introPreset as ClipTemplateDesign['introPreset']
    if (req.outroPreset !== undefined)
      row.outroPreset = req.outroPreset as ClipTemplateDesign['outroPreset']
    if (req.allowedCaptionStyles) row.allowedCaptionStyles = [...req.allowedCaptionStyles.values]
    options.writes?.push({ ...row })
    return create(UpdateVideoTemplateResponseSchema, { template: toProto(row) })
  })
  router.rpc(ClipTemplateService.method.deleteVideoTemplate, (req) => {
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
  router.rpc(ClipGenerationService.method.listClipProjects, () => {
    options.calls?.push('ListClipProjects')
    if (options.projectListFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    return create(ListClipProjectsResponseSchema, {
      projects: [...projects.values()].map((p) => projectProto(options.readProject?.(p) ?? p)),
    })
  })
  router.rpc(ClipGenerationService.method.getClipProject, (req) => {
    options.calls?.push('GetClipProject')
    const p = projects.get(req.id)
    if (!p) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    return create(GetClipProjectResponseSchema, {
      project: projectProto(options.readProject?.(p) ?? p),
    })
  })
  router.rpc(ClipGenerationService.method.finalizeClipProject, async (req) => {
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
      // The plan and the evidence survive finalization and are served as
      // readings (CLIP-160); only the originals behind them are deleted.
    }
    return { project: projectProto(p) }
  })
  router.rpc(ClipGenerationService.method.cancelClipJob, async (req) => {
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
  router.rpc(ClipPlanService.method.getClipCaptionPreview, (req) => {
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
  /** Every intro and outro preset drawn once. The fake writes each slot's numbered label into a
   *  tagged group, not the preset's drawing: what a test can check is that ① offers every preset,
   *  sends its own label and saves the choice — the drawing is pinned by the Go contract. */
  router.rpc(ClipPlanService.method.getClipRegionPresetSamples, (req) => {
    options.calls?.push('GetClipRegionPresetSamples')
    options.regionSampleLabels?.push(req.slotLabel)
    const p = projects.get(req.projectId)
    if (!p) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    if (options.regionSamplesFail) throw connectAppError('CLIP_BUSY', Code.FailedPrecondition)
    const samples = (kind: 'intro' | 'outro') =>
      Object.keys(CLIP_DESIGN.regions[kind]).map((preset) => ({
        preset,
        svg: `<g data-preset="${kind}-${preset}"><text>${req.slotLabel.replace('{n}', '1')}</text></g>`,
        box: { x: 100, y: 800, width: 880, height: 300 },
      }))
    return create(GetClipRegionPresetSamplesResponseSchema, {
      ratio: p.ratio,
      canvas: { x: 0, y: 0, width: 1080, height: 1920 },
      intro: samples('intro'),
      outro: samples('outro'),
    })
  })
  /** Every approved style drawn once. The fake draws a box, not the style: what a test can check
   *  here is that ① offers each style, says which ones are drawn frame by frame, and saves what
   *  was ticked — the drawing itself is the renderer's, pinned by its own Go contract. */
  router.rpc(ClipPlanService.method.getClipCaptionStyleSamples, (req) => {
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
  router.rpc(ClipGenerationService.method.createClipProject, (req) => {
    options.calls?.push('CreateClipProject')
    if (options.projectSaveFails) throw connectAppError('CLIP_INVALID_INPUT', Code.InvalidArgument)
    const template = templateFor(req.videoTemplateId)
    const styles = req.allowedCaptionStyles?.values ?? template?.allowedCaptionStyles
    const p: FakeClipProject = {
      id: `clip-${++next}`,
      title: req.title,
      videoTemplateId: req.videoTemplateId,
      ratio: req.ratio as ClipProjectDraft['ratio'],
      targetDurationMs: req.targetDurationMs,
      disclosure: req.disclosure as ClipProjectDraft['disclosure'],
      hideDisclosure: req.hideDisclosure,
      instruction: req.instruction,
      // A request naming no preset takes the template's selection, or the new-project defaults
      // where there is no template, stored as ids (CLIP-111, CLIP-139, CLIP-168).
      introPreset: (req.introPreset ||
        template?.introPreset ||
        CLIP_DEFAULT_REGION_PRESETS.intro) as ClipProjectDraft['introPreset'],
      outroPreset: (req.outroPreset ||
        template?.outroPreset ||
        CLIP_DEFAULT_REGION_PRESETS.outro) as ClipProjectDraft['outroPreset'],
      ...(styles ? { allowedCaptionStyles: [...styles] } : {}),
    }
    if (req.compositionInputs) {
      const template = rows.get(p.videoTemplateId)!
      const wire = create(ClipProjectSchema, {
        composition: {
          snapshot: {
            version: 1,
            body: template.compositionBody,
            templateId: template.id,
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
  router.rpc(ClipGenerationService.method.updateClipProject, async (req) => {
    options.calls?.push('UpdateClipProject')
    if ((req.introRegion || req.outroRegion) && options.regionSaveGate)
      await options.regionSaveGate()
    if (options.projectSaveGate) await options.projectSaveGate()
    if (options.projectSaveError) throw options.projectSaveError
    if (options.projectSaveFails) throw connectAppError('CLIP_INVALID_INPUT', Code.InvalidArgument)
    const p = projects.get(req.id)
    if (!p) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    // The server bounds the instruction in every Unicode character (clip.BoundedText).
    if (
      req.instruction !== undefined &&
      Array.from(req.instruction).length > CLIP_PROJECT_LIMITS.instruction
    )
      throw connectAppError('CLIP_INVALID_INPUT', Code.InvalidArgument)
    if (req.title !== undefined) p.title = req.title
    // Choosing a template takes its selection in the same write; 없음 moves nothing (CLIP-168).
    const switchedTo =
      req.videoTemplateId !== undefined && req.videoTemplateId !== p.videoTemplateId
        ? templateFor(req.videoTemplateId)
        : undefined
    if (req.videoTemplateId !== undefined) p.videoTemplateId = req.videoTemplateId
    if (req.targetDurationMs !== undefined) p.targetDurationMs = req.targetDurationMs
    if (req.hideDisclosure !== undefined) p.hideDisclosure = req.hideDisclosure
    if (req.disclosure !== undefined)
      p.disclosure = req.disclosure as ClipProjectDraft['disclosure']
    if (req.instruction !== undefined) p.instruction = req.instruction
    if (req.captionPace !== undefined)
      p.captionPace = req.captionPace as ClipProjectDraft['captionPace']
    if (req.accent !== undefined) p.accent = req.accent as ClipProjectDraft['accent']
    if (req.introPreset !== undefined)
      p.introPreset = req.introPreset as ClipProjectDraft['introPreset']
    if (req.outroPreset !== undefined)
      p.outroPreset = req.outroPreset as ClipProjectDraft['outroPreset']
    if (req.allowedCaptionStyles) p.allowedCaptionStyles = [...req.allowedCaptionStyles.values]
    if (switchedTo) {
      p.introPreset = switchedTo.introPreset
      p.outroPreset = switchedTo.outroPreset
      p.allowedCaptionStyles = [...switchedTo.allowedCaptionStyles]
      const seeds = options.templateRegions?.[switchedTo.id]
      if (seeds && p.regions) {
        const regions = p.regions
        const seeded = { ...regions }
        for (const kind of REGION_KINDS) {
          const words = seeds[kind]
          if (!words) continue
          seeded[kind] = {
            enabled: true,
            slots: regions[kind].slots.map((slot, i) =>
              slot.ownerFixed || words[i] === undefined
                ? slot
                : { ...slot, text: words[i], bound: false },
            ),
          }
        }
        p.regions = { ...seeded, revision: regions.revision + 1 }
      }
    }
    if (req.introRegion || req.outroRegion) {
      options.regionWrites?.push({
        expectedRegionRevision: req.expectedRegionRevision,
        ...(req.introRegion
          ? { intro: { enabled: req.introRegion.enabled, slots: req.introRegion.slots } }
          : {}),
        ...(req.outroRegion
          ? { outro: { enabled: req.outroRegion.enabled, slots: req.outroRegion.slots } }
          : {}),
      })
      if (!p.regions) throw connectAppError('CLIP_INVALID_INPUT', Code.InvalidArgument)
      if (
        req.expectedRegionRevision !== undefined &&
        req.expectedRegionRevision !== p.regions.revision
      )
        throw connectAppError('CLIP_PLAN_CONFLICT', Code.Aborted)
      const regions = p.regions
      const patched = { ...regions }
      for (const kind of REGION_KINDS) {
        const edit = kind === 'intro' ? req.introRegion : req.outroRegion
        if (!edit) continue
        patched[kind] = {
          enabled: edit.enabled ?? regions[kind].enabled,
          slots: regions[kind].slots.map((slot) => {
            const change = edit.slots.find((one) => one.id === slot.id)
            if (!change) return slot
            return {
              ...slot,
              ...(change.instruction !== undefined
                ? { instruction: change.instruction, instructionEdited: true }
                : {}),
              ...(change.text !== undefined
                ? { text: change.text, ownerFixed: true, bound: false }
                : {}),
            }
          }),
        }
      }
      if (JSON.stringify(patched) !== JSON.stringify(regions))
        p.regions = { ...patched, revision: regions.revision + 1 }
    }
    // A preset or a slot moves the drawing; the plan follows it (CLIP-188).
    projectFakeRegions(p)
    if (req.storyline) {
      const paragraphs = req.storyline.paragraphs.map((paragraph) => ({
        text: paragraph.text,
        observationIds: [...paragraph.observationIds],
      }))
      options.storylineEdits?.push(paragraphs)
      if (options.storylineEditFails)
        throw connectAppError('CLIP_STORYLINE_INVALID', Code.InvalidArgument)
      if (!p.storyline) throw connectAppError('CLIP_STORYLINE_MISSING', Code.FailedPrecondition)
      const held = new Set(paragraphs.flatMap((paragraph) => paragraph.observationIds))
      const madeWith = [
        ...p.storyline.paragraphs.flatMap((paragraph) => paragraph.observationIds),
        ...p.storyline.takenOutObservationIds,
      ]
      p.storyline = {
        ...p.storyline,
        paragraphs,
        editedByHand: true,
        takenOutObservationIds: [...new Set(madeWith)].filter((id) => !held.has(id)),
      }
    }
    if (req.compositionInputs) {
      // A project with no template freezes the grammar's minimum document, and
      // there is no row to read a body from (CLIP-5, T218).
      const template = p.videoTemplateId ? rows.get(p.videoTemplateId)! : undefined
      const snapshot = !template
        ? (p.composition?.snapshot ?? { version: 1, body: emptyCompositionBody })
        : { version: 1, body: template.compositionBody, templateId: template.id }
      const wire = create(ClipProjectSchema, {
        composition: { snapshot, inputs: req.compositionInputs },
      })
      p.composition = toProjectComposition(wire.composition)
      p.compositionInputs = p.composition?.inputs
    }
    options.projectWrites?.push({ ...p })
    return create(UpdateClipProjectResponseSchema, { project: projectProto(p) })
  })
  router.rpc(ClipGenerationService.method.deleteClipProject, (req) => {
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
  router.rpc(ClipGenerationService.method.listClipAnalysisEligibility, () => {
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
  /** Before narration, quote the whole target whenever the selection admits
   *  frame-by-frame captions. Plan-specific tests supply their exact counts. */
  const sequenceCost = (p?: FakeClipProject) => {
    if (options.sequenceCost) return options.sequenceCost
    const selectedStyles = (p?.allowedCaptionStyles ?? []).filter((style) =>
      (options.sequenceStyles ?? ['word-pop']).includes(style),
    ).length
    const frames = selectedStyles ? Math.ceil(((p?.targetDurationMs ?? 15000) * 30) / 1000) : 0
    return {
      fromPlan: false,
      captions: 0,
      frames,
      addedRenderMs: frames * 30,
      selectedStyles,
    }
  }
  router.rpc(ClipGenerationService.method.quoteClipGeneration, (req) => {
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
      ...options.quoteRecovery,
    })
  })
  // A revision is quoted and started like a generation, but against the SAVED
  // PLAN rather than a batch: the quote binds the plan revision it was taken
  // against, and the narration target pays for one writing call instead of two.
  router.rpc(ClipPlanService.method.quoteClipRevision, (req) => {
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
  router.rpc(ClipPlanService.method.startClipRevision, (req) => {
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
  router.rpc(ClipSourceService.method.reorderClipSources, (req) => {
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
  router.rpc(ClipSourceService.method.setClipSourceOriginalSound, (req) => {
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
  router.rpc(ClipPlanService.method.saveClipEditPlan, async (req) => {
    options.calls?.push('SaveClipEditPlan')
    if (options.planSaveGate) await options.planSaveGate()
    const p = projects.get(req.projectId)
    if (!p?.editing) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    if (options.planSaveConflict || req.expectedRevision !== p.editPlanRevision)
      throw connectAppError('CLIP_PLAN_CONFLICT', Code.Aborted)
    const before = p.editing.plan
    p.editing = toClipEditingState(create(ClipEditingStateSchema, { ...p.editing, plan: req.plan }))
    reconcileFakeRegions(p, before, p.editing.plan)
    options.planWrites?.push({ revision: req.expectedRevision, plan: p.editing.plan })
    p.editPlanRevision = (p.editPlanRevision ?? 0) + 1
    return create(SaveClipEditPlanResponseSchema, { project: projectProto(p) })
  })
  const browserRenders = new Map<
    string,
    { projectId: string; revision: number; cancelled: boolean; stored: boolean; passed: boolean }
  >()
  router.rpc(ClipRenderService.method.cancelClipBrowserRender, (req) => {
    options.calls?.push('CancelClipBrowserRender')
    const render = browserRenders.get(req.renderId)
    if (!render) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
    if (render.stored) return { cancelled: false }
    render.cancelled = true
    return { cancelled: true }
  })
  router.rpc(ClipRenderService.method.prepareClipRenderUpload, (req) => {
    options.calls?.push('PrepareClipRenderUpload')
    const render = browserRenders.get(req.renderId)
    if (!render || render.cancelled)
      throw connectAppError('CLIP_SOURCE_UNAVAILABLE', Code.FailedPrecondition)
    return {
      putUrl: 'https://private.test/browser-put',
      headers: { 'Content-Type': 'video/mp4', 'If-None-Match': '*' },
    }
  })
  router.rpc(ClipRenderService.method.reportClipRenderVerdict, (req) => {
    options.calls?.push('ReportClipRenderVerdict')
    const render = browserRenders.get(req.renderId)
    if (!render || render.cancelled)
      throw connectAppError('CLIP_SOURCE_UNAVAILABLE', Code.FailedPrecondition)
    render.passed = req.passed
    return {
      passed: req.passed,
      notices: req.passed ? [] : [{ code: 'render_output_verdict', action: 'shortfall' }],
    }
  })
  router.rpc(ClipRenderService.method.completeClipRenderUpload, (req) => {
    options.calls?.push('CompleteClipRenderUpload')
    const render = browserRenders.get(req.renderId)
    if (!render || render.cancelled || !render.passed)
      throw connectAppError('CLIP_SOURCE_UNAVAILABLE', Code.FailedPrecondition)
    const p = projects.get(render.projectId)!
    if (p.editPlanRevision !== render.revision)
      throw connectAppError('CLIP_PLAN_CONFLICT', Code.Aborted)
    render.stored = true
    p.renderedPlanRevision = render.revision
    p.result = {
      id: req.renderId,
      renderKind: 'browser',
      contentType: 'video/mp4',
      bytes: 1234,
      durationMs: p.editing!.plan.durationMs,
      createdAt: new Date().toISOString(),
      viewUrl: 'https://private.test/browser-result.mp4',
      downloadUrl: 'https://private.test/browser-download.mp4',
    }
    return { project: projectProto(p) }
  })
  router.rpc(ClipRenderService.method.startClipRender, (req) => {
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
    if (req.renderKind === ClipRenderKind.BROWSER) {
      const renderId = `browser-render-${browserRenders.size + 1}`
      browserRenders.set(renderId, {
        projectId: p.id,
        revision: req.expectedRevision,
        cancelled: false,
        stored: false,
        passed: false,
      })
      return create(StartClipRenderResponseSchema, { renderId })
    }
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
  // 스토리라인 먼저 (CLIP-177): quoted like a generation, one writing call instead of two.
  router.rpc(ClipGenerationService.method.quoteClipStoryline, (req) => {
    options.calls?.push('QuoteClipStoryline')
    options.quoteRequests?.push(req)
    if (options.quoteFails) throw connectAppError(options.quoteFails, Code.FailedPrecondition)
    const batch = batches.get(req.batchId)
    if (!batch || batch.projectId !== req.projectId || batch.state !== 'ready')
      throw connectAppError('CLIP_SOURCE_UNAVAILABLE', Code.FailedPrecondition)
    const q = { id: `quote-${++quoteNumber}`, max: options.quoteMaxCredits ?? 12 }
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
      pricedCalls: [
        { label: 'observe', stage: 'observe', calls: 3 },
        { label: 'storyline', stage: 'write', calls: 1 },
      ],
    })
  })
  // The storyline request (CLIP-181): one writing call, bound to the stored storyline.
  const storylineRequestQuotes = new Map<string, { id: string; max: number; request: string }>()
  router.rpc(ClipGenerationService.method.quoteClipStorylineRevision, (req) => {
    options.calls?.push('QuoteClipStorylineRevision')
    options.storylineRequestQuotes?.push(req)
    const p = projects.get(req.projectId)
    if (!p?.storyline) throw connectAppError('CLIP_STORYLINE_MISSING', Code.FailedPrecondition)
    const q = { id: `quote-${++quoteNumber}`, max: 4, request: req.request }
    storylineRequestQuotes.set(p.id, q)
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
      pricedCalls: [{ label: 'storyline', stage: 'write', calls: 1 }],
    })
  })
  router.rpc(ClipGenerationService.method.startClipStorylineRevision, (req) => {
    options.calls?.push('StartClipStorylineRevision')
    options.storylineRequestStarts?.push(req)
    const p = projects.get(req.projectId)
    const q = p && storylineRequestQuotes.get(p.id)
    if (!p || !q || q.id !== req.quoteId || q.request !== req.request)
      throw connectAppError('CLIP_QUOTE_REQUIRED', Code.FailedPrecondition)
    const jobId = 'clip-storyline-request-job'
    p.latestJob = {
      id: jobId,
      kind: 'revise_storyline_clip',
      status: 'queued',
      stage: 'prepare',
      clipProjectId: p.id,
    }
    return create(StartClipGenerationResponseSchema, { jobId })
  })
  router.rpc(ClipGenerationService.method.startClipStoryline, (req) => {
    options.calls?.push('StartClipStoryline')
    options.storylineStarts?.push(req)
    const p = projects.get(req.projectId)
    const b = batches.get(req.batchId)
    if (!p || !b || b.state !== 'ready')
      throw connectAppError('CLIP_SOURCE_UNAVAILABLE', Code.FailedPrecondition)
    const quote = quotes.get(b.id)
    if (!quote || quote.id !== req.quoteId || quote.max !== req.approvedMaxCredits)
      throw connectAppError('CLIP_QUOTE_REQUIRED', Code.FailedPrecondition)
    b.state = 'consuming'
    const jobId = options.storylineJobId ?? 'clip-storyline-job'
    p.latestJob = {
      id: jobId,
      kind: 'storyline_clip',
      status: 'queued',
      stage: 'prepare',
      clipProjectId: p.id,
    }
    p.latestAttempt = { jobId, batchId: b.id, quoteId: quote.id }
    return create(StartClipGenerationResponseSchema, { jobId })
  })
  router.rpc(ClipGenerationService.method.startClipGeneration, (req) => {
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
  router.rpc(ClipSourceService.method.getClipSources, (req) => {
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
  router.rpc(ClipSourceService.method.getClipSourcePlayback, (req) => {
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
  router.rpc(ClipSourceService.method.createClipSourceBatch, (req) => {
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
  router.rpc(ClipSourceService.method.confirmClipSource, (req) => {
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
  router.rpc(ClipSourceService.method.discardClipSourceBatch, (req) => {
    options.calls?.push('DiscardClipSourceBatch')
    options.sourceRequests?.push(req)
    if (options.discardFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    batches.delete(req.batchId)
    return create(DiscardClipSourceBatchResponseSchema, {})
  })
}
