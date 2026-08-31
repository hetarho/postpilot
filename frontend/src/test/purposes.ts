import { Code, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import {
  CreatePurposeResponseSchema,
  DeletePurposeResponseSchema,
  ListPurposesResponseSchema,
  PurposeSchema,
  PurposeService,
  UpdatePurposeResponseSchema,
} from '@/shared/api'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakePurposeRow {
  id: string
  name: string
  description?: string
  instructions?: string
  /** Posts currently assigned to it, as the server's projection reports. */
  postCount?: number
}

export interface FakePurposesOptions {
  purposes?: FakePurposeRow[]
  /** Make ListPurposes fail. */
  listFails?: boolean
  /** Records every procedure the transport was asked for. */
  calls?: string[]
  /** Records every UpdatePurpose exactly as it arrived, so a test can prove that an edit of
   *  one field carried ONLY that field (spec/policy/purposes.md). */
  updates?: Array<{
    id: string
    name: string | undefined
    description: string | undefined
    instructions: string | undefined
  }>
}

const DEFAULT_AT = '2026-08-28T12:00:00Z'

interface Row {
  id: string
  name: string
  description: string
  instructions: string
  postCount: number
}

export function registerPurposeService(router: ConnectRouter, options: FakePurposesOptions = {}) {
  const { rpc } = router
  const { calls } = options
  let sequence = 0
  const rows = new Map<string, Row>(
    (options.purposes ?? []).map((row) => [
      row.id,
      {
        id: row.id,
        name: row.name,
        description: row.description ?? '',
        instructions: row.instructions ?? '지침',
        postCount: row.postCount ?? 0,
      },
    ]),
  )

  const toProto = (row: Row) =>
    create(PurposeSchema, { ...row, createdAt: DEFAULT_AT, updatedAt: DEFAULT_AT })

  /** Ordered by name then id, like the directory query. */
  const listed = () =>
    [...rows.values()].sort((a, b) =>
      a.name < b.name ? -1 : a.name > b.name ? 1 : a.id < b.id ? -1 : 1,
    )

  function nameTaken(name: string, exceptId: string): boolean {
    return [...rows.values()].some((row) => row.id !== exceptId && row.name === name)
  }

  rpc(PurposeService.method.listPurposes, () => {
    calls?.push('ListPurposes')
    if (options.listFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    return create(ListPurposesResponseSchema, { purposes: listed().map(toProto) })
  })

  rpc(PurposeService.method.createPurpose, (req) => {
    calls?.push('CreatePurpose')
    const name = req.name.trim()
    if (!name) throw connectAppError('PURPOSE_NAME_REQUIRED', Code.InvalidArgument)
    if (!req.instructions.trim())
      throw connectAppError('PURPOSE_INSTRUCTIONS_REQUIRED', Code.InvalidArgument)
    if (nameTaken(name, '')) throw connectAppError('PURPOSE_NAME_TAKEN', Code.AlreadyExists)
    sequence += 1
    const row: Row = {
      id: `purpose-${sequence}`,
      name,
      description: req.description.trim(),
      instructions: req.instructions.trim(),
      postCount: 0,
    }
    rows.set(row.id, row)
    return create(CreatePurposeResponseSchema, { purpose: toProto(row) })
  })

  rpc(PurposeService.method.updatePurpose, (req) => {
    calls?.push('UpdatePurpose')
    options.updates?.push({
      id: req.id,
      name: req.name,
      description: req.description,
      instructions: req.instructions,
    })
    const row = rows.get(req.id)
    if (!row) throw connectAppError('PURPOSE_NOT_FOUND', Code.NotFound)
    // Presence, like the server: an absent field is not part of the edit at all.
    if (req.name !== undefined) {
      const name = req.name.trim()
      if (!name) throw connectAppError('PURPOSE_NAME_REQUIRED', Code.InvalidArgument)
      if (nameTaken(name, row.id)) throw connectAppError('PURPOSE_NAME_TAKEN', Code.AlreadyExists)
      row.name = name
    }
    if (req.description !== undefined) row.description = req.description.trim()
    if (req.instructions !== undefined) {
      const instructions = req.instructions.trim()
      if (!instructions)
        throw connectAppError('PURPOSE_INSTRUCTIONS_REQUIRED', Code.InvalidArgument)
      row.instructions = instructions
    }
    return create(UpdatePurposeResponseSchema, { purpose: toProto(row) })
  })

  rpc(PurposeService.method.deletePurpose, (req) => {
    calls?.push('DeletePurpose')
    const row = rows.get(req.id)
    if (!row) throw connectAppError('PURPOSE_NOT_FOUND', Code.NotFound)
    rows.delete(req.id)
    return create(DeletePurposeResponseSchema, { detachedPosts: row.postCount })
  })
}
