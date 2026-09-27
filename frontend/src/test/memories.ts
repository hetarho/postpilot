import { Code, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import {
  CreateMemoryResponseSchema,
  DeleteMemoryResponseSchema,
  GetMemoryExtractionResponseSchema,
  ListMemoriesResponseSchema,
  MemoryCandidateSchema,
  MemoryService,
  MemorySchema,
  ProtoMemoryKind,
  ResolveMemoryExtractionResponseSchema,
  StartMemoryExtractionResponseSchema,
  UpdateMemoryResponseSchema,
  appFailureFromConnect,
} from '@/shared/api'
import { MEMORY_TEXT_MAX_CHARS } from '@/entities/memory'
import { connectAppError, durableFailure } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakeMemoryRow {
  id: string
  text: string
  /** Omitted means `preference`, the kind that is a candidate for every post. */
  kind?: ProtoMemoryKind
  tags?: string[]
  sourcePostSlugs?: string[]
}

export interface FakeMemoriesOptions {
  /** Given in the SERVER's injection order — most recently used first — so a test can prove the
   *  screen never reorders them. */
  memories?: FakeMemoryRow[]
  /** Make ListMemories fail. */
  listFails?: boolean
  /** Refuse every create as past the account cap: the FailedPrecondition that names the cap and
   *  evicts nothing (MEM-11). */
  createAtCap?: boolean
  calls?: string[]
  /** Records every UpdateMemory exactly as it arrived, so a test can prove a text edit carried
   *  no tags and a facet edit carried no text. */
  updates?: Array<{
    id: string
    text: string | undefined
    kind: ProtoMemoryKind | undefined
    tags: string[] | undefined
  }>
  /** Records every CreateMemory, including the ones an approval sends with its source post. */
  creates?: Array<{
    text: string
    kind: ProtoMemoryKind
    tags: string[]
    sourcePostSlug: string
  }>
  deletions?: string[]
  /** What the extraction job proposes. Omitted, it proposes nothing — the repeatable "this post
   *  yielded nothing" answer (MEM-14). */
  candidates?: Array<{ text: string; kind?: ProtoMemoryKind; tags?: string[] }>
  /** Refuse StartMemoryExtraction the way the credit gate does: nothing is started and no job
   *  row survives (QUOTA-13). */
  extractionRefused?: boolean
  /** Refuse every create whose text is this one, so the per-row refusal path is testable while
   *  its neighbours still save. */
  refuseCreateOf?: string
  /** Every StartMemoryExtraction's post slug. */
  extractions?: string[]
  /** Every ResolveMemoryExtraction: the job and the indexes it approved. */
  resolutions?: Array<{ jobId: string; approved: number[] }>
}

const DEFAULT_AT = '2026-09-20T12:00:00Z'

interface Row {
  id: string
  text: string
  kind: ProtoMemoryKind
  tags: string[]
  sourcePostSlugs: string[]
}

export function registerMemoryService(router: ConnectRouter, options: FakeMemoriesOptions = {}) {
  const { rpc } = router
  const { calls } = options
  let sequence = 0
  const order: string[] = []
  const rows = new Map<string, Row>()
  for (const row of options.memories ?? []) {
    rows.set(row.id, {
      id: row.id,
      text: row.text,
      kind: row.kind ?? ProtoMemoryKind.PREFERENCE,
      tags: row.tags ?? [],
      sourcePostSlugs: row.sourcePostSlugs ?? [],
    })
    order.push(row.id)
  }

  const toProto = (row: Row) =>
    create(MemorySchema, {
      id: row.id,
      text: row.text,
      kind: row.kind,
      tags: row.tags,
      sourcePostSlugs: row.sourcePostSlugs,
      createdAt: DEFAULT_AT,
      updatedAt: DEFAULT_AT,
      lastSeenAt: DEFAULT_AT,
    })

  rpc(MemoryService.method.listMemories, () => {
    calls?.push('ListMemories')
    if (options.listFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    const listed = order.map((id) => rows.get(id)).filter((row): row is Row => row !== undefined)
    return create(ListMemoriesResponseSchema, { memories: listed.map(toProto) })
  })

  const createMemory = (req: {
    text: string
    kind: ProtoMemoryKind
    tags: string[]
    sourcePostSlug: string
  }) => {
    options.creates?.push({
      text: req.text,
      kind: req.kind,
      tags: [...req.tags],
      sourcePostSlug: req.sourcePostSlug,
    })
    const text = req.text.trim()
    if (!text) throw connectAppError('MEMORY_TEXT_REQUIRED', Code.InvalidArgument)
    if ([...text].length > MEMORY_TEXT_MAX_CHARS) {
      throw connectAppError('MEMORY_TEXT_TOO_LONG', Code.InvalidArgument, {
        max: String(MEMORY_TEXT_MAX_CHARS),
        actual: String([...text].length),
      })
    }
    if (req.kind === ProtoMemoryKind.UNSPECIFIED) {
      throw connectAppError('MEMORY_KIND_INVALID', Code.InvalidArgument)
    }
    if (options.createAtCap) {
      throw connectAppError('MEMORY_LIMIT_REACHED', Code.FailedPrecondition, { max: '300' })
    }
    if (options.refuseCreateOf === text) {
      throw connectAppError('MEMORY_TEXT_TAKEN', Code.AlreadyExists)
    }
    // Exact after trim: a second sighting links the post and answers the existing row (MEM-9).
    const existing = [...rows.values()].find((row) => row.text === text)
    if (existing) {
      if (req.sourcePostSlug && !existing.sourcePostSlugs.includes(req.sourcePostSlug)) {
        existing.sourcePostSlugs.push(req.sourcePostSlug)
      }
      return create(CreateMemoryResponseSchema, {
        memory: toProto(existing),
        deduplicated: true,
      })
    }
    sequence += 1
    const row: Row = {
      id: `memory-${sequence}`,
      text,
      kind: req.kind,
      tags: [...req.tags],
      sourcePostSlugs: req.sourcePostSlug ? [req.sourcePostSlug] : [],
    }
    rows.set(row.id, row)
    // A new memory is the most recently used one, which is where the server's order puts it.
    order.unshift(row.id)
    return create(CreateMemoryResponseSchema, { memory: toProto(row), deduplicated: false })
  }
  rpc(MemoryService.method.createMemory, (req) => {
    calls?.push('CreateMemory')
    return createMemory(req)
  })

  rpc(MemoryService.method.updateMemory, (req) => {
    calls?.push('UpdateMemory')
    options.updates?.push({
      id: req.id,
      text: req.text,
      kind: req.kind,
      tags: req.tags ? [...req.tags.tags] : undefined,
    })
    const row = rows.get(req.id)
    if (!row) throw connectAppError('MEMORY_NOT_FOUND', Code.NotFound)
    if (req.text !== undefined) {
      const text = req.text.trim()
      if (!text) throw connectAppError('MEMORY_TEXT_REQUIRED', Code.InvalidArgument)
      if ([...rows.values()].some((other) => other.id !== row.id && other.text === text)) {
        throw connectAppError('MEMORY_TEXT_TAKEN', Code.AlreadyExists)
      }
      row.text = text
    }
    if (req.kind !== undefined) row.kind = req.kind
    if (req.tags) row.tags = [...req.tags.tags]
    return create(UpdateMemoryResponseSchema, { memory: toProto(row) })
  })

  rpc(MemoryService.method.deleteMemory, (req) => {
    calls?.push('DeleteMemory')
    options.deletions?.push(req.id)
    if (!rows.delete(req.id)) throw connectAppError('MEMORY_NOT_FOUND', Code.NotFound)
    const at = order.indexOf(req.id)
    if (at >= 0) order.splice(at, 1)
    return create(DeleteMemoryResponseSchema, {})
  })

  rpc(MemoryService.method.startMemoryExtraction, (req) => {
    calls?.push('StartMemoryExtraction')
    options.extractions?.push(req.postSlug)
    if (options.extractionRefused) {
      throw connectAppError('INSUFFICIENT_CREDITS', Code.ResourceExhausted, {
        required: '1',
        balance: '0',
        renews_at: DEFAULT_AT,
      })
    }
    return create(StartMemoryExtractionResponseSchema, { jobId: 'extract-job' })
  })

  // Once ruled on, the extraction answers nothing: its payload is cleared (MEM-15).
  let resolved = false
  rpc(MemoryService.method.resolveMemoryExtraction, (req) => {
    calls?.push('ResolveMemoryExtraction')
    options.resolutions?.push({ jobId: req.jobId, approved: [...req.approved] })
    if (req.jobId !== 'extract-job' || resolved) {
      throw connectAppError('MEMORY_NOT_FOUND', Code.NotFound)
    }
    const candidates = options.candidates ?? []
    let saved = 0
    const failures: Array<{ index: number; failure: ReturnType<typeof durableFailure> }> = []
    for (const index of req.approved) {
      const candidate = candidates[index]
      if (!candidate) throw connectAppError('MEMORY_NOT_FOUND', Code.InvalidArgument)
      try {
        createMemory({
          text: candidate.text,
          kind: candidate.kind ?? ProtoMemoryKind.PREFERENCE,
          tags: candidate.tags ?? [],
          sourcePostSlug: 'draft',
        })
        saved += 1
      } catch (cause) {
        const failure = appFailureFromConnect(cause)
        failures.push({ index, failure: durableFailure(failure.reason, { ...failure.params }) })
      }
    }
    if (failures.length === 0) resolved = true
    return create(ResolveMemoryExtractionResponseSchema, { saved, failures })
  })

  rpc(MemoryService.method.getMemoryExtraction, (req) => {
    calls?.push('GetMemoryExtraction')
    if (req.jobId !== 'extract-job' || resolved) {
      throw connectAppError('MEMORY_NOT_FOUND', Code.NotFound)
    }
    return create(GetMemoryExtractionResponseSchema, {
      postSlug: 'draft',
      candidates: (options.candidates ?? []).map((candidate) =>
        create(MemoryCandidateSchema, {
          text: candidate.text,
          kind: candidate.kind ?? ProtoMemoryKind.PREFERENCE,
          tags: candidate.tags ?? [],
        }),
      ),
    })
  })
}
