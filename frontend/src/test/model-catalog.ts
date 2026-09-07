// Shared fake ModelCatalogService for tests.
//
// It models the three server behaviours the operator screen depends on: the browse list is the
// provider's catalog merged with what has been curated, a curation write changes that list, and a
// failed provider read degrades to curated rows with `fetchError` set rather than to nothing.
import { Code, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import {
  AdminService,
  ListCatalogResponseSchema,
  ModelCatalogService,
  SetEstimatorComboResponseSchema,
  ProtoPlan,
  SetModelPurposeResponseSchema,
  UpdateModelResponseSchema,
} from '@/shared/api'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakeCatalogEntry {
  modelId: string
  label?: string
  vision?: boolean
  /** The model takes video input — narrower than vision, and checked per run (VIDEO-11). */
  videoInput?: boolean
  structuredOutput?: boolean
  contextTokens?: bigint
  inputUsdPerMillion?: string
  outputUsdPerMillion?: string
  curated?: boolean
  purposes?: string[]
  imageOutput?: boolean
  videoOutput?: boolean
  minPlan?: ProtoPlan
  listed?: boolean
  /** The effort PER PURPOSE, keyed by purpose slug — the server stores it on the
   *  registration, and each purpose tab shows and edits only its own (change 24). */
  reasoningEffort?: Record<string, string>
  /** Recent reasoning spend per purpose slug, as the ledger's aggregate reports it for that
   *  purpose's stage. A purpose absent from the map has no recorded call. */
  reasoningSpend?: Record<
    string,
    {
      calls: bigint
      reasoningTokens: bigint
      completionTokens: bigint
      reasoningTruncations?: bigint
    }
  >
  sourceCreatedAt?: bigint
  /** What the source publishes about this model's reasoning (change 27). Omitted means a
   *  model that reasons but whose accepted values the source does not list — the common
   *  shape, and the one that keeps the control offering all eight values. */
  reasoning?: {
    reasons?: boolean
    efforts?: string[]
    defaultEffort?: string
    mandatory?: boolean
    nativeEffort?: boolean
    maxTokens?: boolean
    drifted?: boolean
  }
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
  /** The estimator assignments the operator has made. Absent combos read as unassigned. */
  estimatorCombos?: Array<{ combo: string; observeModelId?: string; writeModelId?: string }>
  /** Refuse SetEstimatorCombo the way an unregistered model does. */
  comboWriteFails?: boolean
  calls?: string[]
}

export function registerModelCatalogService(
  router: ConnectRouter,
  options: FakeModelCatalogOptions = {},
) {
  const { rpc } = router
  const { calls } = options
  let entries = [...(options.entries ?? [])]
  let combos = [...(options.estimatorCombos ?? [])]

  // The listing is per purpose: it reports each entry's effort for THAT purpose and attaches
  // that stage's spend signal, so a test can prove a tab shows its own value and not another's.
  const answer = (purpose: string) =>
    create(ListCatalogResponseSchema, {
      entries: entries.map((entry) => ({
        modelId: entry.modelId,
        providerSlug: entry.modelId.split('/')[0] ?? entry.modelId,
        label: entry.label ?? entry.modelId,
        description: '',
        vision: entry.vision ?? false,
        videoInput: entry.videoInput ?? false,
        structuredOutput: entry.structuredOutput ?? false,
        contextTokens: entry.contextTokens ?? 0n,
        inputUsdPerMillion: entry.inputUsdPerMillion ?? '',
        outputUsdPerMillion: entry.outputUsdPerMillion ?? '',
        curated: entry.curated ?? false,
        purposes: entry.purposes ?? [],
        imageOutput: entry.imageOutput ?? false,
        videoOutput: entry.videoOutput ?? false,
        listed: entry.listed ?? true,
        reasoningEffort: entry.reasoningEffort?.[purpose] ?? '',
        reasoningSpend: entry.reasoningSpend?.[purpose]
          ? {
              ...entry.reasoningSpend[purpose],
              reasoningTruncations: entry.reasoningSpend[purpose]?.reasoningTruncations ?? 0n,
            }
          : undefined,
        reasons: entry.reasoning?.reasons ?? true,
        reasoningEfforts: entry.reasoning?.efforts ?? [],
        reasoningDefaultEffort: entry.reasoning?.defaultEffort ?? '',
        reasoningMandatory: entry.reasoning?.mandatory ?? false,
        reasoningNativeEffort: entry.reasoning?.nativeEffort ?? false,
        reasoningMaxTokens: entry.reasoning?.maxTokens ?? false,
        // The server derives drift from the stored override against the live list; the fake
        // does the same so a test cannot set a flag the data contradicts.
        reasoningDrifted: driftedFrom(
          entry.reasoningEffort?.[purpose] ?? '',
          entry.reasoning?.efforts ?? [],
        ),
        // Like the server: an entry read live from the source is known by construction, and a
        // failed fetch serves stored rows whose capability may predate this data.
        reasoningKnown: !options.fetchFails,
        sourceCreatedAt: entry.sourceCreatedAt ?? 0n,
      })),
      fetchedAt: options.fetchFails ? '' : (options.fetchedAt ?? '2026-09-03T09:00:00Z'),
      fromCache: options.fromCache ?? false,
      fetchError: options.fetchFails ? 'the provider catalog could not be read' : '',
      // All four in ladder order, the way the server answers, so an unassigned tier is a
      // rendered state rather than a missing row.
      estimatorCombos: ['quality', 'balanced', 'value', 'cheapest'].map((combo) => {
        const assigned = combos.find((entry) => entry.combo === combo)
        return {
          combo,
          observeModelId: assigned?.observeModelId ?? '',
          writeModelId: assigned?.writeModelId ?? '',
        }
      }),
    })

  rpc(ModelCatalogService.method.listCatalog, (req) => {
    calls?.push(
      `${req.refresh ? 'ListCatalog:refresh' : 'ListCatalog'}${req.purpose ? `:${req.purpose}` : ''}`,
    )
    if (options.listFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    return answer(req.purpose)
  })

  // The assignment's write is an AdminService method but it lives here, with the read it
  // changes: two fakes cannot share one piece of state, and the operator's screen reads the
  // assignment back out of ListCatalog.
  rpc(AdminService.method.setEstimatorCombo, (req) => {
    calls?.push(`SetEstimatorCombo:${req.combo}:${req.observeModelId}/${req.writeModelId}`)
    if (options.comboWriteFails) {
      throw connectAppError('MODEL_NOT_REGISTERED', Code.FailedPrecondition)
    }
    const registered = (modelId: string, purpose: string) =>
      (entries.find((entry) => entry.modelId === modelId)?.purposes ?? []).includes(purpose)
    if (
      !registered(req.observeModelId, 'photo-analysis') ||
      !registered(req.writeModelId, 'writing')
    ) {
      throw connectAppError('MODEL_NOT_REGISTERED', Code.FailedPrecondition)
    }
    combos = [
      ...combos.filter((entry) => entry.combo !== req.combo),
      { combo: req.combo, observeModelId: req.observeModelId, writeModelId: req.writeModelId },
    ]
    return create(SetEstimatorComboResponseSchema, {})
  })

  rpc(ModelCatalogService.method.setModelPurpose, (req) => {
    calls?.push(
      `SetModelPurpose:${req.purpose}:${req.registered ? 'register' : 'deregister'}:${req.modelId}`,
    )
    if (options.writeFails) throw connectAppError('MODEL_NOT_FOUND', Code.NotFound)
    entries = entries.map((entry) => {
      if (entry.modelId !== req.modelId) return entry
      const others = (entry.purposes ?? []).filter((purpose) => purpose !== req.purpose)
      return {
        ...entry,
        curated: true,
        purposes: req.registered ? [...others, req.purpose] : others,
      }
    })
    const written = entries.find((entry) => entry.modelId === req.modelId)
    return create(SetModelPurposeResponseSchema, {
      entry: {
        modelId: req.modelId,
        curated: true,
        purposes: written?.purposes ?? [],
        listed: written?.listed ?? true,
      },
    })
  })

  rpc(ModelCatalogService.method.updateModel, (req) => {
    calls?.push(`UpdateModel:${req.purpose}:${req.reasoningEffort ?? ''}`)
    if (options.writeFails) throw connectAppError('MODEL_NOT_FOUND', Code.NotFound)
    // The server refuses an effort for a purpose the model does not serve; the fake holds the
    // same rule so a test cannot pass against a looser server than the real one. The reason
    // is generic on purpose: like MODEL_PURPOSE_INELIGIBLE, this refusal is unreachable
    // through the operator UI — the control appears only once registered — so change 24 left
    // the normalized reason set alone rather than adding copy for it.
    const target = entries.find((entry) => entry.modelId === req.modelId)
    if (!target || !(target.purposes ?? []).includes(req.purpose)) {
      throw connectAppError('UNKNOWN_FAILURE', Code.FailedPrecondition)
    }
    entries = entries.map((entry) =>
      entry.modelId === req.modelId
        ? {
            ...entry,
            reasoningEffort: {
              ...entry.reasoningEffort,
              [req.purpose]: req.reasoningEffort ?? entry.reasoningEffort?.[req.purpose] ?? '',
            },
          }
        : entry,
    )
    const written = entries.find((entry) => entry.modelId === req.modelId)
    return create(UpdateModelResponseSchema, {
      entry: {
        modelId: req.modelId,
        curated: true,
        purposes: written?.purposes ?? [],
        reasoningEffort: written?.reasoningEffort?.[req.purpose] ?? '',
        reasons: written?.reasoning?.reasons ?? true,
        reasoningEfforts: written?.reasoning?.efforts ?? [],
        reasoningDefaultEffort: written?.reasoning?.defaultEffort ?? '',
        reasoningMandatory: written?.reasoning?.mandatory ?? false,
        reasoningNativeEffort: written?.reasoning?.nativeEffort ?? false,
        reasoningMaxTokens: written?.reasoning?.maxTokens ?? false,
        reasoningDrifted: driftedFrom(
          written?.reasoningEffort?.[req.purpose] ?? '',
          written?.reasoning?.efforts ?? [],
        ),
        reasoningKnown: true,
      },
    })
  })
}

/** The server's own drift rule: an override the model's published list no longer contains.
 *  An empty list is unknown, so nothing drifts from it. */
function driftedFrom(effort: string, efforts: readonly string[]): boolean {
  if (effort === '' || effort === 'unset' || efforts.length === 0) return false
  return !efforts.includes(effort)
}
