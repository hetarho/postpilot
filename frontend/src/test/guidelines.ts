import { Code, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import {
  CreateGuidelineResponseSchema,
  DeleteGuidelineResponseSchema,
  GuidelineSchema,
  GuidelineService,
  ListGuidelinesResponseSchema,
  ProtoGuidelineScope,
  UpdateGuidelineResponseSchema,
} from '@/shared/api'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakeGuidelineRow {
  id: string
  text: string
  /** Omitted means 전역. A `purposes` scope with an empty array is the orphaned state. */
  purposeRefs?: Array<{ id: string; name: string }>
  scope?: 'global' | 'purposes'
}

export interface FakeGuidelinesOptions {
  guidelines?: FakeGuidelineRow[]
  /** Make ListGuidelines fail. */
  listFails?: boolean
  /** Refuse every create as an exact duplicate, the AlreadyExists path the capture treats as
   *  information rather than a failure. */
  createDuplicates?: boolean
  calls?: string[]
  /** Records every UpdateGuideline exactly as it arrived, so a test can prove a text edit
   *  carried no scope and a scope patch carried no text (spec/policy/guidelines.md). */
  updates?: Array<{
    id: string
    text: string | undefined
    scope: { scope: ProtoGuidelineScope; purposeIds: string[] } | undefined
  }>
  /** Records every CreateGuideline, including the ones the capture dialog sends. */
  creates?: Array<{ text: string; scope: ProtoGuidelineScope; purposeIds: string[] }>
}

const DEFAULT_AT = '2026-09-01T12:00:00Z'

interface Row {
  id: string
  text: string
  scope: 'global' | 'purposes'
  purposes: Array<{ id: string; name: string }>
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
      scope: row.scope ?? (row.purposeRefs ? 'purposes' : 'global'),
      purposes: row.purposeRefs ?? [],
    })
    order.push(row.id)
  }

  const toProto = (row: Row) =>
    create(GuidelineSchema, {
      id: row.id,
      text: row.text,
      scope: row.scope === 'global' ? ProtoGuidelineScope.GLOBAL : ProtoGuidelineScope.PURPOSES,
      purposes: row.purposes,
      createdAt: DEFAULT_AT,
      updatedAt: DEFAULT_AT,
    })

  /** Injection order: the global group first, then the scoped group, each in creation order —
   *  exactly what the server returns, so a test can assert the screen never reorders it. */
  const listed = () => {
    const all = order.map((id) => rows.get(id)).filter((row): row is Row => row !== undefined)
    return [
      ...all.filter((row) => row.scope === 'global'),
      ...all.filter((row) => row.scope !== 'global'),
    ]
  }

  rpc(GuidelineService.method.listGuidelines, () => {
    calls?.push('ListGuidelines')
    if (options.listFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    return create(ListGuidelinesResponseSchema, { guidelines: listed().map(toProto) })
  })

  rpc(GuidelineService.method.createGuideline, (req) => {
    calls?.push('CreateGuideline')
    options.creates?.push({ text: req.text, scope: req.scope, purposeIds: [...req.purposeIds] })
    const text = req.text.trim()
    if (!text) throw connectAppError('GUIDELINE_TEXT_REQUIRED', Code.InvalidArgument)
    if (options.createDuplicates || [...rows.values()].some((row) => row.text === text)) {
      throw connectAppError('GUIDELINE_TEXT_TAKEN', Code.AlreadyExists)
    }
    const scoped = req.scope === ProtoGuidelineScope.PURPOSES
    if (scoped === (req.purposeIds.length === 0)) {
      throw connectAppError('GUIDELINE_SCOPE_INVALID', Code.InvalidArgument)
    }
    sequence += 1
    const row: Row = {
      id: `guideline-${sequence}`,
      text,
      scope: scoped ? 'purposes' : 'global',
      purposes: req.purposeIds.map((id) => ({ id, name: id })),
    }
    rows.set(row.id, row)
    order.push(row.id)
    return create(CreateGuidelineResponseSchema, { guideline: toProto(row) })
  })

  rpc(GuidelineService.method.updateGuideline, (req) => {
    calls?.push('UpdateGuideline')
    options.updates?.push({
      id: req.id,
      text: req.text,
      scope: req.scope
        ? { scope: req.scope.scope, purposeIds: [...req.scope.purposeIds] }
        : undefined,
    })
    const row = rows.get(req.id)
    if (!row) throw connectAppError('GUIDELINE_NOT_FOUND', Code.NotFound)
    // Presence, like the server: an absent part is not part of the edit at all.
    if (req.text !== undefined) {
      const text = req.text.trim()
      if (!text) throw connectAppError('GUIDELINE_TEXT_REQUIRED', Code.InvalidArgument)
      row.text = text
    }
    if (req.scope !== undefined) {
      const scoped = req.scope.scope === ProtoGuidelineScope.PURPOSES
      if (scoped === (req.scope.purposeIds.length === 0)) {
        throw connectAppError('GUIDELINE_SCOPE_INVALID', Code.InvalidArgument)
      }
      row.scope = scoped ? 'purposes' : 'global'
      row.purposes = req.scope.purposeIds.map((id) => ({ id, name: id }))
    }
    return create(UpdateGuidelineResponseSchema, { guideline: toProto(row) })
  })

  rpc(GuidelineService.method.deleteGuideline, (req) => {
    calls?.push('DeleteGuideline')
    if (!rows.delete(req.id)) throw connectAppError('GUIDELINE_NOT_FOUND', Code.NotFound)
    return create(DeleteGuidelineResponseSchema, {})
  })
}
