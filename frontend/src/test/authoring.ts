import { create, type MessageInitShape } from '@bufbuild/protobuf'
import { Code, createClient, type Transport, type createRouterTransport } from '@connectrpc/connect'
import {
  ConfigurationAuthoringService as Service,
  TemplateService,
  GuidelineService,
  ClipTemplateService,
  ProtoConfigurationKind,
  ProtoAuthoringDraftState,
  ProtoAuthoringMode,
  ProtoGuidelineKind,
  ProtoGuidelineScope,
  ProtoBlogField,
} from '@/shared/api'
import { canSaveTemplate, TEMPLATE_PARSE_OPTIONS, parseTemplate } from '@/entities/template'
import { canSaveGuideline, remainingGuidelineTitleChars } from '@/entities/guideline'
import { BLOG_FIELD_IDS, type BlogFieldId } from '@/entities/blog-field'
import { parseClipTemplate } from '@/entities/clip-template'
import { connectAppError } from './app-error'
import { toWire, fromWire } from './wire-enum'

type Router = Parameters<Parameters<typeof createRouterTransport>[0]>[0]
type Wire = NonNullable<
  Awaited<
    ReturnType<ReturnType<typeof createClient<typeof Service>>['getAuthoringSession']>
  >['session']
>
type Artifact = NonNullable<Wire['workingSource']>
type WireInit = NonNullable<
  MessageInitShape<typeof Service.method.getAuthoringSession.output>['session']
>
type SummaryInit = NonNullable<
  MessageInitShape<typeof Service.method.listAuthoringSummaries.output>['summaries']
>[number]
export interface FakeAuthoringOptions {
  sessions?: WireInit[]
  summaries?: SummaryInit[]
  summaryFails?: boolean
  saveConflict?: boolean
  createGate?: Promise<void>
  calls?: string[]
  creates?: Array<{ kind: ProtoConfigurationKind; targetId: string; referencePost: string }>
  patches?: Artifact[]
  saves?: Artifact[]
  starts?: Array<{
    prompt: string
    mode: ProtoAuthoringMode
    source?: Artifact
    referencePost: string
  }>
}
const number = (text: string | undefined) =>
  text === undefined || text === '' ? undefined : Number(text)
const domainValue = (source: Artifact) =>
  JSON.stringify({ ...source, id: '', revision: 0, builderState: '' })
const wire = (fields: WireInit) =>
  create(Service.method.getAuthoringSession.output, { session: fields }).session!

/** Real UI actors use this private durable server fixture. Canonical writes still go through
 * the existing fake domain services so their caps, duplicates and fixture receipts remain visible. */
export function registerAuthoringService(
  router: Router,
  options: FakeAuthoringOptions,
  getTransport: () => Transport,
) {
  const sessions = new Map(
    (options.sessions ?? []).map((fields) => {
      const session = wire(fields)
      return [session.id, session]
    }),
  )
  const references = new Map<string, string>()
  const versions = new Map<string, string>()
  const receipts = new Map<string, Wire>()
  let sequence = 0
  const calls = options.calls
  const read = (id: string) => {
    const session = sessions.get(id)
    if (!session) throw connectAppError('AUTHORING_SESSION_NOT_FOUND', Code.NotFound)
    return session
  }
  const capture = async (
    kind: ProtoConfigurationKind,
    id: string,
  ): Promise<Artifact | undefined> => {
    if (!id) return
    const transport = getTransport()
    if (kind === ProtoConfigurationKind.POST_TEMPLATE) {
      const row = (await createClient(TemplateService, transport).listTemplates({})).templates.find(
        (row) => row.id === id,
      )
      if (!row) throw connectAppError('TEMPLATE_NOT_FOUND', Code.NotFound)
      return create(Service.method.getAuthoringSession.output, {
        session: {
          workingSource: {
            id,
            name: row.name,
            description: row.description,
            body: row.body,
            titleArea: row.titleArea,
            targetLength: row.targetLength?.toString() ?? '',
            tagCount: row.tagCount?.toString() ?? '',
          },
        },
      }).session!.workingSource!
    }
    if (kind === ProtoConfigurationKind.VIDEO_TEMPLATE) {
      const row = (
        await createClient(ClipTemplateService, transport).listVideoTemplates({})
      ).templates.find((row) => row.id === id)
      if (!row) throw connectAppError('CLIP_NOT_FOUND', Code.NotFound)
      return create(Service.method.getAuthoringSession.output, {
        session: { workingSource: { id, name: row.name, body: row.compositionBody } },
      }).session!.workingSource!
    }
    if (
      kind === ProtoConfigurationKind.POST_GUIDELINE ||
      kind === ProtoConfigurationKind.VIDEO_GUIDELINE
    ) {
      const row = (
        await createClient(GuidelineService, transport).listGuidelines({
          kind:
            kind === ProtoConfigurationKind.POST_GUIDELINE
              ? ProtoGuidelineKind.POST
              : ProtoGuidelineKind.CLIP,
        })
      ).guidelines.find((row) => row.id === id)
      if (!row) throw connectAppError('GUIDELINE_NOT_FOUND', Code.NotFound)
      return create(Service.method.getAuthoringSession.output, {
        session: {
          workingSource: {
            id,
            name: row.title,
            body: row.text,
            scope:
              row.scope === ProtoGuidelineScope.TEMPLATES
                ? 'templates'
                : row.scope === ProtoGuidelineScope.FIELDS
                  ? 'fields'
                  : 'global',
            templateIds: row.templates.map((row) => row.id),
            fields: row.fields.map(
              (field) => fromWire(ProtoBlogField, field, BLOG_FIELD_IDS) ?? '',
            ),
          },
        },
      }).session!.workingSource!
    }
  }
  const valid = (session: Wire, source: Artifact) => {
    if (session.kind === ProtoConfigurationKind.POST_TEMPLATE)
      return (
        canSaveTemplate(source) &&
        parseTemplate(source.titleArea, source.body, TEMPLATE_PARSE_OPTIONS).ok &&
        [source.targetLength, source.tagCount].every(
          (value) => value === undefined || value === '' || /^\d+$/.test(value),
        ) &&
        (source.targetLength === undefined ||
          source.targetLength === '' ||
          (Number(source.targetLength) >= 100 && Number(source.targetLength) <= 10000)) &&
        (source.tagCount === undefined ||
          source.tagCount === '' ||
          (Number(source.tagCount) >= 1 && Number(source.tagCount) <= 10))
      )
    if (
      session.kind === ProtoConfigurationKind.POST_GUIDELINE ||
      session.kind === ProtoConfigurationKind.VIDEO_GUIDELINE
    )
      return (
        remainingGuidelineTitleChars(source.name) >= 0 &&
        canSaveGuideline(source.body, {
          kind: source.scope === 'templates' || source.scope === 'fields' ? source.scope : 'global',
          templateIds: source.templateIds,
          fields: source.fields as BlogFieldId[],
        }) &&
        !!source.scope &&
        !(session.kind === ProtoConfigurationKind.VIDEO_GUIDELINE && source.scope === 'fields')
      )
    if (session.kind === ProtoConfigurationKind.VIDEO_TEMPLATE) {
      try {
        parseClipTemplate(source.body)
        return !!source.name.trim() && Array.from(source.name.trim()).length <= 40
      } catch {
        return false
      }
    }
    return !!source.body.trim()
  }
  const response = (session?: Wire) => ({ session })
  router.rpc(Service.method.listAuthoringSummaries, (req) => {
    calls?.push('ListAuthoringSummaries')
    if (options.summaryFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    const all =
      options.summaries ??
      [...sessions.values()].reverse().map((session) => ({
        sessionId: session.id,
        kind: session.kind,
        targetId: session.targetId,
        displayName:
          session.workingSource?.name || session.savedBaseline?.name || session.workingSource?.body,
        revision: session.revision,
        savedAvailable: session.savedAvailable,
        hasUnpublishedChanges: session.hasUnpublishedChanges,
        activeJobId: session.activeJobId,
        publicationPending: session.phase === 'saving',
        targetConflict: session.failureReason === 'AUTHORING_SAVE_CONFLICT',
        draftState: session.draftState,
        lastPublication: session.saved,
        updatedAt: '2026-10-07T00:00:00Z',
      }))
    return create(Service.method.listAuthoringSummaries.output, {
      summaries: all.filter(
        (row) => row.kind === req.kind && (!req.unsavedOnly || !row.savedAvailable),
      ),
    })
  })
  router.rpc(Service.method.getLatestAuthoringSession, (req) => {
    calls?.push('GetLatestAuthoringSession')
    return response(
      [...sessions.values()]
        .reverse()
        .find(
          (session) =>
            session.kind === req.kind &&
            (session.targetId === req.targetId || session.saved?.id === req.targetId),
        ),
    )
  })
  router.rpc(Service.method.getAuthoringSession, (req) => {
    calls?.push('GetAuthoringSession')
    return response(read(req.sessionId))
  })
  router.rpc(Service.method.createAuthoringSession, async (req) => {
    calls?.push('CreateAuthoringSession')
    options.creates?.push({
      kind: req.kind,
      targetId: req.targetId,
      referencePost: req.referencePost,
    })
    await options.createGate
    const replay = receipts.get(req.requestId)
    if (replay) return response(replay)
    const baseline = await capture(req.kind, req.targetId)
    const source =
      baseline ??
      create(Service.method.getAuthoringSession.output, {
        session: {
          workingSource: {
            id: 'draft',
            body: req.kind === ProtoConfigurationKind.VIDEO_TEMPLATE ? '<clip version="1"/>' : '',
            targetLength: req.kind === ProtoConfigurationKind.POST_TEMPLATE ? '' : undefined,
            tagCount: req.kind === ProtoConfigurationKind.POST_TEMPLATE ? '' : undefined,
            scope:
              req.kind === ProtoConfigurationKind.POST_GUIDELINE ||
              req.kind === ProtoConfigurationKind.VIDEO_GUIDELINE
                ? 'global'
                : undefined,
          },
        },
      }).session!.workingSource!
    const session = wire({
      id: `authoring-${++sequence}`,
      kind: req.kind,
      targetId: req.targetId,
      targetVersion: 'version',
      revision: 1,
      phase: req.targetId ? 'editing' : 'choosing',
      workingSource: source,
      savedBaseline: baseline,
      selected: baseline,
      draftState: baseline ? ProtoAuthoringDraftState.VALID : ProtoAuthoringDraftState.INCOMPLETE,
      savedAvailable: !!baseline,
      candidateCount: 8,
    })
    sessions.set(session.id, session)
    references.set(session.id, req.referencePost)
    if (baseline) versions.set(session.id, domainValue(baseline))
    receipts.set(req.requestId, session)
    return response(session)
  })
  router.rpc(Service.method.patchAuthoringDraft, (req) => {
    calls?.push('PatchAuthoringDraft')
    const session = read(req.sessionId)
    const replay = receipts.get(req.operationKey)
    if (replay) return response(replay)
    if (req.expectedRevision !== session.revision)
      throw connectAppError('AUTHORING_REVISION_CONFLICT', Code.Aborted)
    if (!req.workingSource) throw connectAppError('AUTHORING_DRAFT_INVALID', Code.InvalidArgument)
    options.patches?.push(req.workingSource)
    const source = req.workingSource
    session.revision++
    session.workingSource = source
    session.phase = 'editing'
    session.hasUnpublishedChanges =
      !session.savedBaseline || domainValue(source) !== domainValue(session.savedBaseline)
    session.draftState = valid(session, source)
      ? ProtoAuthoringDraftState.VALID
      : source.body.trim()
        ? ProtoAuthoringDraftState.INVALID
        : ProtoAuthoringDraftState.INCOMPLETE
    if (session.draftState === ProtoAuthoringDraftState.VALID) session.selected = source
    receipts.set(req.operationKey, wire(session))
    return response(session)
  })
  router.rpc(Service.method.resetAuthoringChat, (req) => {
    calls?.push('ResetAuthoringChat')
    const session = read(req.sessionId)
    session.revision++
    session.turns = []
    session.pendingRequest = ''
    return response(session)
  })
  router.rpc(Service.method.resetAuthoringBaseline, (req) => {
    calls?.push('ResetAuthoringBaseline')
    const session = read(req.sessionId)
    if (!session.savedBaseline)
      throw connectAppError('AUTHORING_DRAFT_INVALID', Code.InvalidArgument)
    session.revision++
    session.workingSource = session.savedBaseline
    session.selected = session.savedBaseline
    session.hasUnpublishedChanges = false
    session.draftState = ProtoAuthoringDraftState.VALID
    return response(session)
  })
  router.rpc(Service.method.estimateAuthoringOperation, () => {
    calls?.push('EstimateAuthoringOperation')
    return { free: false, credits: 1n }
  })
  router.rpc(Service.method.startAuthoringOperation, (req) => {
    calls?.push('StartAuthoringOperation')
    const session = read(req.sessionId)
    options.starts?.push({
      prompt: req.prompt,
      mode: req.mode,
      source: session.workingSource,
      referencePost: references.get(session.id) ?? '',
    })
    session.revision++
    const base = session.workingSource!
    if (req.mode === ProtoAuthoringMode.RECOMMEND) {
      session.candidateCount = req.candidateCount
      session.candidates = Array.from({ length: req.candidateCount }, (_, index) => ({
        ...base,
        id: `candidate-${index}`,
        name: `제안 ${index + 1}`,
        body:
          session.kind === ProtoConfigurationKind.POST_TEMPLATE
            ? `<write>주제 ${index + 1}</write>`
            : session.kind === ProtoConfigurationKind.VIDEO_TEMPLATE
              ? '<clip version="1"/>'
              : `실제 경험을 바탕으로 ${index + 1}번째 방향으로 써 주세요.`,
      }))
      session.phase = 'choosing'
    } else {
      const source = {
        ...base,
        body: valid(session, base) ? base.body : session.selected?.body || '새 지침',
        builderState: '',
      }
      session.workingSource = source
      session.selected = source
      session.draftState = ProtoAuthoringDraftState.VALID
      session.phase = 'editing'
      session.hasUnpublishedChanges = true
      session.turns.push(
        wire({
          turns: [
            {
              id: 'turn',
              request: req.prompt,
              reply: '고친 내용을 확인해 주세요.',
              jobId: 'authoring-job',
              status: 'done',
            },
          ],
        }).turns[0],
      )
    }
    return { jobId: 'authoring-job', session }
  })
  router.rpc(Service.method.selectAuthoringCandidate, (req) => {
    calls?.push('SelectAuthoringCandidate')
    const session = read(req.sessionId)
    const chosen = session.candidates.find((row) => row.id === req.candidateId)
    if (!chosen) throw connectAppError('AUTHORING_DRAFT_INVALID', Code.InvalidArgument)
    session.selected = chosen
    session.workingSource = chosen
    session.revision++
    session.phase = 'editing'
    session.hasUnpublishedChanges = true
    session.draftState = ProtoAuthoringDraftState.VALID
    return response(session)
  })
  router.rpc(Service.method.saveAuthoringSession, async (req) => {
    calls?.push('SaveAuthoringSession')
    const session = read(req.sessionId)
    const replay = receipts.get(req.operationKey)
    if (replay) return response(replay)
    const source = session.workingSource!
    if (!valid(session, source))
      throw connectAppError('AUTHORING_DRAFT_INVALID', Code.InvalidArgument)
    if (
      options.saveConflict ||
      (session.targetId &&
        domainValue((await capture(session.kind, session.targetId))!) !== versions.get(session.id))
    ) {
      session.failureReason = 'AUTHORING_SAVE_CONFLICT'
      throw connectAppError('AUTHORING_SAVE_CONFLICT', Code.Aborted)
    }
    options.saves?.push(source)
    const transport = getTransport()
    let id = session.targetId
    const outcome = id ? 'updated' : 'created'
    if (session.kind === ProtoConfigurationKind.POST_TEMPLATE) {
      const client = createClient(TemplateService, transport)
      const fields = {
        name: source.name,
        description: source.description,
        body: source.body,
        titleArea: source.titleArea,
        targetLength: number(source.targetLength),
        tagCount: number(source.tagCount),
      }
      const result = id
        ? await client.updateTemplate({ id, ...fields })
        : await client.createTemplate(fields)
      id = result.template!.id
    } else if (session.kind === ProtoConfigurationKind.VIDEO_TEMPLATE) {
      const client = createClient(ClipTemplateService, transport)
      const fields = { name: source.name, compositionBody: source.body }
      const result = id
        ? await client.updateVideoTemplate({ id, ...fields })
        : await client.createVideoTemplate(fields)
      id = result.template!.id
    } else {
      const client = createClient(GuidelineService, transport)
      const scope =
        source.scope === 'templates'
          ? ProtoGuidelineScope.TEMPLATES
          : source.scope === 'fields'
            ? ProtoGuidelineScope.FIELDS
            : ProtoGuidelineScope.GLOBAL
      const fields = {
        title: source.name,
        text: source.body,
        scope,
        templateIds: source.templateIds,
        fields: source.fields.map((field) => toWire(ProtoBlogField, field)),
      }
      const result = id
        ? await client.updateGuideline({
            id,
            title: source.name,
            text: source.body,
            scope: { scope, templateIds: fields.templateIds, fields: fields.fields },
          })
        : await client.createGuideline({
            ...fields,
            kind:
              session.kind === ProtoConfigurationKind.POST_GUIDELINE
                ? ProtoGuidelineKind.POST
                : ProtoGuidelineKind.CLIP,
          })
      id = result.guideline!.id
    }
    session.revision++
    session.saved = wire({
      saved: { kind: session.kind, id, name: source.name || source.body, outcome },
    }).saved!
    session.phase = 'saved'
    session.savedBaseline = { ...source, builderState: '' }
    session.workingSource = session.savedBaseline
    session.selected = session.savedBaseline
    session.hasUnpublishedChanges = false
    session.savedAvailable = true
    versions.set(session.id, domainValue(session.savedBaseline))
    receipts.set(req.operationKey, wire(session))
    return response(session)
  })
  router.rpc(Service.method.cancelAuthoringOperation, (req) => {
    calls?.push('CancelAuthoringOperation')
    const session = read(req.sessionId)
    session.activeJobId = ''
    session.phase = 'editing'
    return response(session)
  })
}
