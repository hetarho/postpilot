import { Code, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import {
  CreateTemplateResponseSchema,
  DeleteTemplateResponseSchema,
  ListTemplatesResponseSchema,
  TemplateSchema,
  TemplateService,
  UpdateTemplateResponseSchema,
} from '@/shared/api'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakeTemplateRow {
  id: string
  name: string
  description?: string
  body?: string
  /** Posts currently assigned to it, as the server's projection reports. */
  postCount?: number
}

export interface FakeTemplatesOptions {
  templates?: FakeTemplateRow[]
  /** Make ListTemplates fail. */
  listFails?: boolean
  /** Records every procedure the transport was asked for. */
  calls?: string[]
  /** Records every UpdateTemplate exactly as it arrived, so a test can prove that an edit of
   *  one field carried ONLY that field (spec/policy/templates.md). */
  updates?: Array<{
    id: string
    name: string | undefined
    description: string | undefined
    body: string | undefined
  }>
}

const DEFAULT_AT = '2026-08-28T12:00:00Z'

interface Row {
  id: string
  name: string
  description: string
  body: string
  postCount: number
}

export function registerTemplateService(router: ConnectRouter, options: FakeTemplatesOptions = {}) {
  const { rpc } = router
  const { calls } = options
  let sequence = 0
  const rows = new Map<string, Row>(
    (options.templates ?? []).map((row) => [
      row.id,
      {
        id: row.id,
        name: row.name,
        description: row.description ?? '',
        body: row.body ?? '지침',
        postCount: row.postCount ?? 0,
      },
    ]),
  )

  const toProto = (row: Row) =>
    create(TemplateSchema, { ...row, createdAt: DEFAULT_AT, updatedAt: DEFAULT_AT })

  /** Ordered by name then id, like the directory query. */
  const listed = () =>
    [...rows.values()].sort((a, b) =>
      a.name < b.name ? -1 : a.name > b.name ? 1 : a.id < b.id ? -1 : 1,
    )

  function nameTaken(name: string, exceptId: string): boolean {
    return [...rows.values()].some((row) => row.id !== exceptId && row.name === name)
  }

  rpc(TemplateService.method.listTemplates, () => {
    calls?.push('ListTemplates')
    if (options.listFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    return create(ListTemplatesResponseSchema, { templates: listed().map(toProto) })
  })

  rpc(TemplateService.method.createTemplate, (req) => {
    calls?.push('CreateTemplate')
    const name = req.name.trim()
    if (!name) throw connectAppError('TEMPLATE_NAME_REQUIRED', Code.InvalidArgument)
    if (!req.body.trim()) throw connectAppError('TEMPLATE_BODY_REQUIRED', Code.InvalidArgument)
    if (nameTaken(name, '')) throw connectAppError('TEMPLATE_NAME_TAKEN', Code.AlreadyExists)
    sequence += 1
    const row: Row = {
      id: `template-${sequence}`,
      name,
      description: req.description.trim(),
      body: req.body.trim(),
      postCount: 0,
    }
    rows.set(row.id, row)
    return create(CreateTemplateResponseSchema, { template: toProto(row) })
  })

  rpc(TemplateService.method.updateTemplate, (req) => {
    calls?.push('UpdateTemplate')
    options.updates?.push({
      id: req.id,
      name: req.name,
      description: req.description,
      body: req.body,
    })
    const row = rows.get(req.id)
    if (!row) throw connectAppError('TEMPLATE_NOT_FOUND', Code.NotFound)
    // Presence, like the server: an absent field is not part of the edit at all.
    if (req.name !== undefined) {
      const name = req.name.trim()
      if (!name) throw connectAppError('TEMPLATE_NAME_REQUIRED', Code.InvalidArgument)
      if (nameTaken(name, row.id)) throw connectAppError('TEMPLATE_NAME_TAKEN', Code.AlreadyExists)
      row.name = name
    }
    if (req.description !== undefined) row.description = req.description.trim()
    if (req.body !== undefined) {
      const body = req.body.trim()
      if (!body) throw connectAppError('TEMPLATE_BODY_REQUIRED', Code.InvalidArgument)
      row.body = body
    }
    return create(UpdateTemplateResponseSchema, { template: toProto(row) })
  })

  rpc(TemplateService.method.deleteTemplate, (req) => {
    calls?.push('DeleteTemplate')
    const row = rows.get(req.id)
    if (!row) throw connectAppError('TEMPLATE_NOT_FOUND', Code.NotFound)
    rows.delete(req.id)
    return create(DeleteTemplateResponseSchema, { detachedPosts: row.postCount })
  })
}
