import { Code, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import {
  CreateGuidelineResponseSchema,
  DeleteGuidelineResponseSchema,
  DismissGuidelineCandidateResponseSchema,
  GuidelineCandidateSchema,
  GuidelinePresetSchema,
  GuidelineSchema,
  GuidelineService,
  ListGuidelineCandidatesResponseSchema,
  ListGuidelinesResponseSchema,
  ProtoGuidelineScope,
  ProtoBlogField,
  UpdateGuidelinePresetResponseSchema,
  UpdateGuidelineResponseSchema,
} from '@/shared/api'
import { BLOG_FIELD_IDS, type BlogFieldId } from '@/entities/blog-field'
import { GUIDELINE_TEXT_MAX_CHARS } from '@/entities/guideline'
import { connectAppError } from './app-error'
import { fromWire, toWire } from './wire-enum'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakeGuidelineRow {
  id: string
  text: string
  /** A `templates` scope with an empty array is the orphaned state. */
  templateRefs?: Array<{ id: string; name: string }>
  /** The 분야 set of a `fields` scope. */
  fields?: BlogFieldId[]
  /** Omitted, the scope reads as `fields` when `fields` are given, as `templates` when
   *  `templateRefs` are, and otherwise as 전역. */
  scope?: 'global' | 'templates' | 'fields'
  /** A scope number sent as-is instead of the kind's own — for a value this build cannot read. */
  wireScope?: number
}

export interface FakeGuidelineCandidateRow {
  id: string
  text: string
  /** Omitted or empty means the source post is gone — the text survives, the link does not. */
  postSlug?: string
  /** Omitted means 1. Above 1 it is what the 후보 row shows as "N번 요청함". */
  occurrences?: number
  lastSeenAt?: string
}

export interface FakeGuidelinesOptions {
  guidelines?: FakeGuidelineRow[]
  /** Pending candidates, given in the SERVER's review order so a test can prove the screen never
   *  reorders them. */
  candidates?: FakeGuidelineCandidateRow[]
  /** The pending queue is at its server-side bound: further revisions record nothing. */
  candidateQueueFull?: boolean
  /** Make ListGuidelineCandidates fail, which the screen renders as no section rather than a
   *  second error region. */
  candidateListFails?: boolean
  /** Refuse every create as past the account cap, the FailedPrecondition an approval must keep
   *  the candidate pending through. */
  createAtCap?: boolean
  /** Make ListGuidelines fail. */
  listFails?: boolean
  /** Refuse every create as an exact duplicate, the AlreadyExists path the capture treats as
   *  information rather than a failure. */
  createDuplicates?: boolean
  /** Refuse every 분야-scoped create and update as naming a 분야 the server does not know. */
  refuseFields?: boolean
  calls?: string[]
  /** Records every UpdateGuideline exactly as it arrived, so a test can prove a text edit
   *  carried no scope and a scope patch carried no text (spec/legacy/policy/guidelines.md). */
  updates?: Array<{
    id: string
    text: string | undefined
    scope:
      { scope: ProtoGuidelineScope; templateIds: string[]; fields: ProtoBlogField[] } | undefined
  }>
  /** Records every CreateGuideline, including the ones the capture dialog and an approval send.
   *  `fromCandidateId` is present only when the approved candidate's text was edited first. */
  creates?: Array<{
    text: string
    scope: ProtoGuidelineScope
    templateIds: string[]
    fields: ProtoBlogField[]
    fromCandidateId?: string
  }>
  /** Records every DismissGuidelineCandidate. */
  dismissals?: string[]
  /** The preset's switch and 분야. Omitted means off with none, where every account starts. */
  preset?: { enabled?: boolean; fields?: BlogFieldId[] }
  /** Records every UpdateGuidelinePreset exactly as it arrived, absent halves as undefined. */
  presetUpdates?: Array<{ enabled: boolean | undefined; fields: ProtoBlogField[] | undefined }>
  /** Refuse every preset save as naming a 분야 the server does not know. */
  presetUpdateFails?: boolean
  /** Holds every preset save until the test releases it, so the pending state is observable. */
  presetUpdateGate?: Promise<void>
}

/** The preset's text as this fake serves it. The real one is the server's, and never edited. */
export const FAKE_GUIDELINE_PRESET_TEXT =
  '[분야 상위 글 문구]\n원문이 이미 같은 뜻으로 쓴 자리에서만 아래 문구로 바꿔 쓴다.'

const DEFAULT_AT = '2026-09-01T12:00:00Z'

interface Row {
  id: string
  text: string
  scope: 'global' | 'templates' | 'fields'
  templates: Array<{ id: string; name: string }>
  fields: BlogFieldId[]
  wireScope?: number
}

const SCOPE_TO_PROTO: Record<Row['scope'], ProtoGuidelineScope> = {
  global: ProtoGuidelineScope.GLOBAL,
  templates: ProtoGuidelineScope.TEMPLATES,
  fields: ProtoGuidelineScope.FIELDS,
}

/** The server's shape rules (GUIDE-5): each kind carries its own set, never the other kind's, and
 *  a narrowed kind carries at least one. An unset scope is refused, never defaulted. */
function scopeKind(
  scope: ProtoGuidelineScope,
  templateIds: string[],
  fields: ProtoBlogField[],
): Row['scope'] {
  const valid =
    scope === ProtoGuidelineScope.GLOBAL
      ? templateIds.length === 0 && fields.length === 0
      : scope === ProtoGuidelineScope.TEMPLATES
        ? templateIds.length > 0 && fields.length === 0
        : scope === ProtoGuidelineScope.FIELDS
          ? fields.length > 0 && templateIds.length === 0
          : false
  if (!valid) throw connectAppError('GUIDELINE_SCOPE_INVALID', Code.InvalidArgument)
  if (fields.some((field) => !fromWire(ProtoBlogField, field, BLOG_FIELD_IDS)))
    throw connectAppError('GUIDELINE_FIELD_NOT_FOUND', Code.NotFound)
  return scope === ProtoGuidelineScope.GLOBAL
    ? 'global'
    : scope === ProtoGuidelineScope.TEMPLATES
      ? 'templates'
      : 'fields'
}

function toFieldIds(fields: ProtoBlogField[]): BlogFieldId[] {
  return fields
    .map((field) => fromWire(ProtoBlogField, field, BLOG_FIELD_IDS))
    .filter((field): field is BlogFieldId => field !== undefined)
}

export function registerGuidelineService(
  router: ConnectRouter,
  options: FakeGuidelinesOptions = {},
) {
  const { rpc } = router
  const { calls } = options
  let sequence = 0
  const order: string[] = []
  const rows = new Map<string, Row>()
  for (const row of options.guidelines ?? []) {
    rows.set(row.id, {
      id: row.id,
      text: row.text,
      scope: row.scope ?? (row.fields ? 'fields' : row.templateRefs ? 'templates' : 'global'),
      templates: row.templateRefs ?? [],
      fields: row.fields ?? [],
      wireScope: row.wireScope,
    })
    order.push(row.id)
  }

  const toProto = (row: Row) =>
    create(GuidelineSchema, {
      id: row.id,
      text: row.text,
      scope: row.wireScope ?? SCOPE_TO_PROTO[row.scope],
      templates: row.templates,
      fields: row.fields.map((field) => toWire(ProtoBlogField, field)),
      createdAt: DEFAULT_AT,
      updatedAt: DEFAULT_AT,
    })

  /** Injection order: the global group, then the template group, then the 분야 group, each in
   *  creation order — exactly what the server returns (GUIDE-14), so a test can assert the screen
   *  never reorders it. */
  const listed = () => {
    const all = order.map((id) => rows.get(id)).filter((row): row is Row => row !== undefined)
    return [
      ...all.filter((row) => row.scope === 'global'),
      ...all.filter((row) => row.scope === 'templates'),
      ...all.filter((row) => row.scope === 'fields'),
    ]
  }

  // The product's preset: one per account, always answered with the list (GUIDE-29).
  const preset = {
    enabled: options.preset?.enabled ?? false,
    fields: options.preset?.fields ?? [],
  }
  const presetProto = () =>
    create(GuidelinePresetSchema, {
      text: FAKE_GUIDELINE_PRESET_TEXT,
      enabled: preset.enabled,
      fields: preset.fields.map((field) => toWire(ProtoBlogField, field)),
    })

  // Candidates (change 26). Declared before the create handler because a create is also an
  // approval: the server marks the candidate in the same transaction, so the fake must too.
  const candidates = new Map<string, FakeGuidelineCandidateRow>()
  for (const candidate of options.candidates ?? []) candidates.set(candidate.id, candidate)

  rpc(GuidelineService.method.listGuidelines, () => {
    calls?.push('ListGuidelines')
    if (options.listFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    return create(ListGuidelinesResponseSchema, {
      guidelines: listed().map(toProto),
      preset: presetProto(),
    })
  })

  rpc(GuidelineService.method.updateGuidelinePreset, async (req) => {
    calls?.push('UpdateGuidelinePreset')
    await options.presetUpdateGate
    options.presetUpdates?.push({
      enabled: req.enabled,
      fields: req.fields ? [...req.fields.fields] : undefined,
    })
    if (options.presetUpdateFails) throw connectAppError('GUIDELINE_FIELD_NOT_FOUND', Code.NotFound)
    // Presence, like the server: an absent half keeps what is stored.
    if (req.fields !== undefined) {
      const fields = toFieldIds(req.fields.fields)
      if (fields.length !== req.fields.fields.length)
        throw connectAppError('GUIDELINE_FIELD_NOT_FOUND', Code.NotFound)
      preset.fields = fields
    }
    if (req.enabled !== undefined) preset.enabled = req.enabled
    return create(UpdateGuidelinePresetResponseSchema, { preset: presetProto() })
  })

  rpc(GuidelineService.method.createGuideline, (req) => {
    calls?.push('CreateGuideline')
    options.creates?.push({
      text: req.text,
      scope: req.scope,
      templateIds: [...req.templateIds],
      fields: [...req.fields],
      fromCandidateId: req.fromCandidateId,
    })
    const text = req.text.trim()
    if (!text) throw connectAppError('GUIDELINE_TEXT_REQUIRED', Code.InvalidArgument)
    // The server's bound, mirrored so an over-long approval is refused rather than truncated.
    if ([...text].length > GUIDELINE_TEXT_MAX_CHARS) {
      throw connectAppError('GUIDELINE_TEXT_TOO_LONG', Code.InvalidArgument, {
        max: String(GUIDELINE_TEXT_MAX_CHARS),
        actual: String([...text].length),
      })
    }
    if (options.createAtCap) {
      throw connectAppError('GUIDELINE_LIMIT_REACHED', Code.FailedPrecondition, { max: '100' })
    }
    if (options.createDuplicates || [...rows.values()].some((row) => row.text === text)) {
      throw connectAppError('GUIDELINE_TEXT_TAKEN', Code.AlreadyExists)
    }
    const scope = scopeKind(req.scope, req.templateIds, req.fields)
    if (options.refuseFields && scope === 'fields')
      throw connectAppError('GUIDELINE_FIELD_NOT_FOUND', Code.NotFound)
    sequence += 1
    const row: Row = {
      id: `guideline-${sequence}`,
      text,
      scope,
      templates: req.templateIds.map((id) => ({ id, name: id })),
      fields: toFieldIds(req.fields),
    }
    rows.set(row.id, row)
    order.push(row.id)
    // The approval half: by id when one was named, and by text either way — which is what marks
    // the candidate an on-the-spot 지침으로 저장 recorded, without the client knowing its id.
    if (req.fromCandidateId) candidates.delete(req.fromCandidateId)
    for (const [id, candidate] of candidates) {
      if (candidate.text.trim() === text) candidates.delete(id)
    }
    return create(CreateGuidelineResponseSchema, { guideline: toProto(row) })
  })

  rpc(GuidelineService.method.updateGuideline, (req) => {
    calls?.push('UpdateGuideline')
    options.updates?.push({
      id: req.id,
      text: req.text,
      scope: req.scope
        ? {
            scope: req.scope.scope,
            templateIds: [...req.scope.templateIds],
            fields: [...req.scope.fields],
          }
        : undefined,
    })
    const row = rows.get(req.id)
    if (!row) throw connectAppError('GUIDELINE_NOT_FOUND', Code.NotFound)
    // Presence, like the server: an absent part is not part of the edit at all. The scope is
    // validated before the text is written, so a refused patch changes nothing.
    const scope = req.scope && scopeKind(req.scope.scope, req.scope.templateIds, req.scope.fields)
    if (options.refuseFields && scope === 'fields')
      throw connectAppError('GUIDELINE_FIELD_NOT_FOUND', Code.NotFound)
    if (req.text !== undefined) {
      const text = req.text.trim()
      if (!text) throw connectAppError('GUIDELINE_TEXT_REQUIRED', Code.InvalidArgument)
      row.text = text
    }
    if (req.scope && scope) {
      row.scope = scope
      row.templates = req.scope.templateIds.map((id) => ({ id, name: id }))
      row.fields = toFieldIds(req.scope.fields)
    }
    return create(UpdateGuidelineResponseSchema, { guideline: toProto(row) })
  })

  rpc(GuidelineService.method.deleteGuideline, (req) => {
    calls?.push('DeleteGuideline')
    if (!rows.delete(req.id)) throw connectAppError('GUIDELINE_NOT_FOUND', Code.NotFound)
    return create(DeleteGuidelineResponseSchema, {})
  })

  // The list is served in the order it was given: the review order is the server's, and a test
  // asserting it proves the screen does not reorder.
  rpc(GuidelineService.method.listGuidelineCandidates, () => {
    calls?.push('ListGuidelineCandidates')
    if (options.candidateListFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    return create(ListGuidelineCandidatesResponseSchema, {
      candidates: [...candidates.values()].map((candidate) =>
        create(GuidelineCandidateSchema, {
          id: candidate.id,
          text: candidate.text,
          postSlug: candidate.postSlug ?? '',
          occurrences: candidate.occurrences ?? 1,
          firstSeenAt: DEFAULT_AT,
          lastSeenAt: candidate.lastSeenAt ?? DEFAULT_AT,
        }),
      ),
      queueFull: options.candidateQueueFull ?? false,
    })
  })

  rpc(GuidelineService.method.dismissGuidelineCandidate, (req) => {
    calls?.push('DismissGuidelineCandidate')
    options.dismissals?.push(req.id)
    // Marked, not deleted, like the server — but the pending LIST is what the screen reads, so
    // the row leaves it either way.
    if (!candidates.delete(req.id)) {
      throw connectAppError('GUIDELINE_CANDIDATE_NOT_FOUND', Code.NotFound)
    }
    return create(DismissGuidelineCandidateResponseSchema, {})
  })
}
