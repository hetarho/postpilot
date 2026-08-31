// Shared fake ProviderService for tests.
//
// It models the server rules the frontend depends on (spec/policy/providers.md): a saved
// model that is not registered comes back `missing` once and is cleared, and a disabled
// or unregistered model cannot be saved.
import { Code, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import {
  ApplyRecommendationSetResponseSchema,
  type AppFailureReason,
  GetSelectionsResponseSchema,
  GetComparisonPairsResponseSchema,
  ListRecommendationSetsResponseSchema,
  ComparisonPairSchema,
  ListModelsResponseSchema,
  ModelInfoSchema,
  ModelRefSchema,
  ProtoPlan,
  ProviderService,
  SaveComparisonPairResponseSchema,
  SaveSelectionResponseSchema,
  SelectionSchema,
  SelectionSlot,
  Stage,
} from '@/shared/api'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakeModel {
  providerId: string
  modelId: string
  label?: string
  vision?: boolean
  structuredOutput?: boolean
  /** The reason the model is disabled; undefined means enabled. */
  disabledReason?: string
  /** The tier the model requires. Defaults to `free`, so a test that says nothing about
   *  plans gets a registry every account can run. */
  minPlan?: ProtoPlan
  /** What the server computed for THIS caller — the model's floor is above their tier. */
  locked?: boolean
}

export interface FakeSelection {
  stage: Stage
  providerId: string
  modelId: string
}

export interface FakeProviderMutationFailure {
  reason: AppFailureReason
  code?: Code
  params?: Record<string, string>
}

export interface FakeProvidersOptions {
  models?: FakeModel[]
  /** Saved choices; one whose model is not in `models` is reported missing and cleared. */
  selections?: FakeSelection[]
  /** Make ListModels fail. */
  listFails?: boolean
  /** Make SaveSelection fail. */
  saveFails?: boolean
  /** Return a structured application failure from SaveSelection. */
  saveFailure?: FakeProviderMutationFailure
  /** Return a structured application failure from SaveComparisonPair. */
  savePairFailure?: FakeProviderMutationFailure
  /** Return a structured application failure from ApplyRecommendationSet. */
  applyRecommendationFailure?: FakeProviderMutationFailure
  /** Hold SaveSelection open until this promise settles. */
  saveGate?: Promise<void>
  /** Records every procedure the transport was asked for. */
  calls?: string[]
  /** Report saved selections whose model is `locked` as missing WITHOUT clearing them, the
   *  way the server treats a plan-locked choice: a downgrade is reversible state. */
  lockedSelections?: boolean
  comparisonPairs?: Array<{
    stage: Stage
    candidateA: { providerId: string; modelId: string }
    candidateB: { providerId: string; modelId: string }
  }>
}

export function registerProviderService(router: ConnectRouter, options: FakeProvidersOptions = {}) {
  const { rpc } = router
  const { calls } = options
  const models = options.models ?? []
  let selections = [...(options.selections ?? [])]
  let comparisonPairs = [...(options.comparisonPairs ?? [])]

  const registered = (providerId: string, modelId: string) =>
    models.find((model) => model.providerId === providerId && model.modelId === modelId)

  rpc(ProviderService.method.listModels, () => {
    calls?.push('ListModels')
    if (options.listFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    return create(ListModelsResponseSchema, {
      models: models.map((model) =>
        create(ModelInfoSchema, {
          ref: { providerId: model.providerId, modelId: model.modelId },
          label: model.label ?? model.modelId,
          vision: model.vision ?? false,
          structuredOutput: model.structuredOutput ?? false,
          disabled: model.disabledReason !== undefined,
          disabledReason: model.disabledReason ?? '',
          minPlan: model.minPlan ?? ProtoPlan.FREE,
          locked: model.locked ?? false,
        }),
      ),
    })
  })

  const lockedFor = (selection: FakeSelection) =>
    Boolean(options.lockedSelections && registered(selection.providerId, selection.modelId)?.locked)

  rpc(ProviderService.method.getSelections, () => {
    calls?.push('GetSelections')
    const answer = selections.map((selection) =>
      create(SelectionSchema, {
        stage: selection.stage,
        ref: { providerId: selection.providerId, modelId: selection.modelId },
        missing: !registered(selection.providerId, selection.modelId) || lockedFor(selection),
      }),
    )
    // Like the server: a vanished choice is told once, then it is gone — but a plan-locked one
    // is kept, because an upgrade must restore it with no re-selection.
    selections = selections.filter(
      (selection) => registered(selection.providerId, selection.modelId) || lockedFor(selection),
    )
    return create(GetSelectionsResponseSchema, { selections: answer })
  })

  rpc(ProviderService.method.saveSelection, async (req) => {
    calls?.push('SaveSelection')
    await options.saveGate
    if (options.saveFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
    if (options.saveFailure) throwFakeMutationFailure(options.saveFailure)
    if (req.stage === Stage.UNSPECIFIED)
      throw connectAppError('MODEL_STAGE_REQUIRED', Code.InvalidArgument)
    const ref = req.ref ?? create(ModelRefSchema, {})
    const model = registered(ref.providerId, ref.modelId)
    if (!model) throw connectAppError('MODEL_NOT_REGISTERED', Code.NotFound)
    if (model.disabledReason !== undefined) {
      throw connectAppError('MODEL_DISABLED', Code.FailedPrecondition)
    }
    // Like the server: the client's rendering is never the gate.
    if (model.locked) {
      throw connectAppError('MODEL_LOCKED', Code.PermissionDenied, {
        model: `${ref.providerId}/${ref.modelId}`,
        models: `${ref.providerId}/${ref.modelId}`,
        required_plan: 'max',
      })
    }
    selections = [
      ...selections.filter((selection) => selection.stage !== req.stage),
      { stage: req.stage, providerId: ref.providerId, modelId: ref.modelId },
    ]
    return create(SaveSelectionResponseSchema, {
      selection: create(SelectionSchema, { stage: req.stage, ref }),
    })
  })

  rpc(ProviderService.method.getComparisonPairs, () =>
    create(GetComparisonPairsResponseSchema, {
      pairs: comparisonPairs.map((pair) =>
        create(ComparisonPairSchema, {
          stage: pair.stage,
          candidateA: create(SelectionSchema, {
            stage: pair.stage,
            slot: SelectionSlot.CANDIDATE_A,
            ref: pair.candidateA,
          }),
          candidateB: create(SelectionSchema, {
            stage: pair.stage,
            slot: SelectionSlot.CANDIDATE_B,
            ref: pair.candidateB,
          }),
        }),
      ),
    }),
  )
  rpc(ProviderService.method.listRecommendationSets, () =>
    create(ListRecommendationSetsResponseSchema, {}),
  )
  rpc(ProviderService.method.saveComparisonPair, (request) => {
    calls?.push('SaveComparisonPair')
    if (options.savePairFailure) throwFakeMutationFailure(options.savePairFailure)
    const candidateA = request.candidateA ?? create(ModelRefSchema, {})
    const candidateB = request.candidateB ?? create(ModelRefSchema, {})
    comparisonPairs = [
      ...comparisonPairs.filter((pair) => pair.stage !== request.stage),
      { stage: request.stage, candidateA, candidateB },
    ]
    return create(SaveComparisonPairResponseSchema, {
      pair: create(ComparisonPairSchema, {
        stage: request.stage,
        candidateA: create(SelectionSchema, {
          stage: request.stage,
          slot: SelectionSlot.CANDIDATE_A,
          ref: candidateA,
        }),
        candidateB: create(SelectionSchema, {
          stage: request.stage,
          slot: SelectionSlot.CANDIDATE_B,
          ref: candidateB,
        }),
      }),
    })
  })
  rpc(ProviderService.method.applyRecommendationSet, () => {
    calls?.push('ApplyRecommendationSet')
    if (options.applyRecommendationFailure) {
      throwFakeMutationFailure(options.applyRecommendationFailure)
    }
    return create(ApplyRecommendationSetResponseSchema, {})
  })
}

function throwFakeMutationFailure(failure: FakeProviderMutationFailure): never {
  throw connectAppError(failure.reason, failure.code ?? Code.InvalidArgument, failure.params)
}

/** A transport serving only ProviderService — for tests of the catalog hooks/components. */
export function createFakeProviderTransport(options: FakeProvidersOptions = {}) {
  return createRouterTransport((router) => registerProviderService(router, options))
}
