// Shared fake ProviderService for tests.
//
// It models the server rules the frontend depends on (spec/policy/providers.md): a saved
// model that is not registered comes back `missing` once and is cleared, and a disabled
// or unregistered model cannot be saved.
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import {
  GetSelectionsResponseSchema,
  ListModelsResponseSchema,
  ModelInfoSchema,
  ModelRefSchema,
  ProviderService,
  SaveSelectionResponseSchema,
  SelectionSchema,
  Stage,
} from '@/shared/api'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakeModel {
  providerId: string
  modelId: string
  label?: string
  vision?: boolean
  structuredOutput?: boolean
  /** The reason the model is disabled; undefined means enabled. */
  disabledReason?: string
}

export interface FakeSelection {
  stage: Stage
  providerId: string
  modelId: string
}

export interface FakeProvidersOptions {
  models?: FakeModel[]
  /** Saved choices; one whose model is not in `models` is reported missing and cleared. */
  selections?: FakeSelection[]
  /** Make ListModels fail. */
  listFails?: boolean
  /** Make SaveSelection fail. */
  saveFails?: boolean
  /** Records every procedure the transport was asked for. */
  calls?: string[]
}

export function registerProviderService(router: ConnectRouter, options: FakeProvidersOptions = {}) {
  const { rpc } = router
  const { calls } = options
  const models = options.models ?? []
  let selections = [...(options.selections ?? [])]

  const registered = (providerId: string, modelId: string) =>
    models.find((model) => model.providerId === providerId && model.modelId === modelId)

  rpc(ProviderService.method.listModels, () => {
    calls?.push('ListModels')
    if (options.listFails) throw new ConnectError('unavailable', Code.Unavailable)
    return create(ListModelsResponseSchema, {
      models: models.map((model) =>
        create(ModelInfoSchema, {
          ref: { providerId: model.providerId, modelId: model.modelId },
          label: model.label ?? model.modelId,
          vision: model.vision ?? false,
          structuredOutput: model.structuredOutput ?? false,
          disabled: model.disabledReason !== undefined,
          disabledReason: model.disabledReason ?? '',
        }),
      ),
    })
  })

  rpc(ProviderService.method.getSelections, () => {
    calls?.push('GetSelections')
    const answer = selections.map((selection) =>
      create(SelectionSchema, {
        stage: selection.stage,
        ref: { providerId: selection.providerId, modelId: selection.modelId },
        missing: !registered(selection.providerId, selection.modelId),
      }),
    )
    // Like the server: a vanished choice is told once, then it is gone.
    selections = selections.filter((selection) => registered(selection.providerId, selection.modelId))
    return create(GetSelectionsResponseSchema, { selections: answer })
  })

  rpc(ProviderService.method.saveSelection, (req) => {
    calls?.push('SaveSelection')
    if (options.saveFails) throw new ConnectError('unavailable', Code.Unavailable)
    if (req.stage === Stage.UNSPECIFIED) throw new ConnectError('stage is required', Code.InvalidArgument)
    const ref = req.ref ?? create(ModelRefSchema, {})
    const model = registered(ref.providerId, ref.modelId)
    if (!model) throw new ConnectError('model not registered', Code.NotFound)
    if (model.disabledReason !== undefined) {
      throw new ConnectError('model disabled', Code.FailedPrecondition)
    }
    selections = [
      ...selections.filter((selection) => selection.stage !== req.stage),
      { stage: req.stage, providerId: ref.providerId, modelId: ref.modelId },
    ]
    return create(SaveSelectionResponseSchema, {
      selection: create(SelectionSchema, { stage: req.stage, ref }),
    })
  })
}

/** A transport serving only ProviderService — for tests of the catalog hooks/components. */
export function createFakeProviderTransport(options: FakeProvidersOptions = {}) {
  return createRouterTransport((router) => registerProviderService(router, options))
}
