// Shared fake ModelCatalogService for tests.
//
// It models the three server behaviours the operator screen depends on: the browse list is the
// provider's catalog merged with what has been curated, a curation write changes that list, and a
// failed provider read degrades to curated rows with `fetchError` set rather than to nothing.
import { Code, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import {
  EnableModelResponseSchema,
  ListCatalogResponseSchema,
  ModelCatalogService,
  ProtoPlan,
  UpdateModelResponseSchema,
} from '@/shared/api'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakeCatalogEntry {
  modelId: string
  label?: string
  vision?: boolean
  structuredOutput?: boolean
  contextTokens?: bigint
  inputUsdPerMillion?: string
  outputUsdPerMillion?: string
  curated?: boolean
  enabled?: boolean
  minPlan?: ProtoPlan
  listed?: boolean
  reasoningEffort?: string
  sourceCreatedAt?: bigint
}

export interface FakeModelCatalogOptions {
  entries?: FakeCatalogEntry[]
  /** Answer as though the provider's catalog could not be read. */
  fetchFails?: boolean
  /** Report the snapshot as served from the server's cache. */
  fromCache?: boolean
  fetchedAt?: string
  /** Refuse every curation write, the way an unknown model does. */
  writeFails?: boolean
  listFails?: boolean
  calls?: string[]
}

export function registerModelCatalogService(
  router: ConnectRouter,
  options: FakeModelCatalogOptions = {},
) {
  const { rpc } = router
  const { calls } = options
  let entries = [...(options.entries ?? [])]

  const answer = () =>
    create(ListCatalogResponseSchema, {
      entries: entries.map((entry) => ({
        modelId: entry.modelId,
        providerSlug: entry.modelId.split('/')[0] ?? entry.modelId,
        label: entry.label ?? entry.modelId,
        description: '',
        vision: entry.vision ?? false,
        structuredOutput: entry.structuredOutput ?? false,
        contextTokens: entry.contextTokens ?? 0n,
        inputUsdPerMillion: entry.inputUsdPerMillion ?? '',
        outputUsdPerMillion: entry.outputUsdPerMillion ?? '',
        curated: entry.curated ?? false,
        enabled: entry.enabled ?? false,
        listed: entry.listed ?? true,
        reasoningEffort: entry.reasoningEffort ?? '',
        sourceCreatedAt: entry.sourceCreatedAt ?? 0n,
      })),
      fetchedAt: options.fetchFails ? '' : (options.fetchedAt ?? '2026-09-03T09:00:00Z'),
      fromCache: options.fromCache ?? false,
      fetchError: options.fetchFails ? 'the provider catalog could not be read' : '',
    })

  rpc(ModelCatalogService.method.listCatalog, (req) => {
    calls?.push(req.refresh ? 'ListCatalog:refresh' : 'ListCatalog')
    if (options.listFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    return answer()
  })

  rpc(ModelCatalogService.method.enableModel, (req) => {
    calls?.push('EnableModel')
    if (options.writeFails) throw connectAppError('MODEL_NOT_FOUND', Code.NotFound)
    entries = entries.map((entry) =>
      entry.modelId === req.modelId ? { ...entry, curated: true, enabled: true } : entry,
    )
    const written = entries.find((entry) => entry.modelId === req.modelId)
    return create(EnableModelResponseSchema, {
      entry: {
        modelId: req.modelId,
        curated: true,
        enabled: true,
        listed: written?.listed ?? true,
      },
    })
  })

  rpc(ModelCatalogService.method.updateModel, (req) => {
    calls?.push('UpdateModel')
    if (options.writeFails) throw connectAppError('MODEL_NOT_FOUND', Code.NotFound)
    entries = entries.map((entry) =>
      entry.modelId === req.modelId
        ? {
            ...entry,
            enabled: req.enabled ?? entry.enabled,
            reasoningEffort: req.reasoningEffort ?? entry.reasoningEffort,
          }
        : entry,
    )
    return create(UpdateModelResponseSchema, {
      entry: { modelId: req.modelId, curated: true, enabled: req.enabled ?? true },
    })
  })
}
